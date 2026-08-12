package bootstrap

import (
	"context"
	"fmt"
	"net/http"

	bloodinfra "dispatch/internal/modules/blood/infrastructure"
	bloodworkers "dispatch/internal/modules/blood/workers"
	notificationsapp "dispatch/internal/modules/notifications/application"
	notificationsinfra "dispatch/internal/modules/notifications/infrastructure"

	"github.com/IBM/sarama"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"dispatch/internal/platform/config"
	"dispatch/internal/platform/events"
	"dispatch/internal/shared/types"
)

type App struct {
	cfg   config.Config
	log   *zap.Logger
	http  *http.Server
	kafka events.KafkaBus
}

type Worker struct {
	Config config.Config
	Logger *zap.Logger
	Bus    events.KafkaBus
	Group  sarama.ConsumerGroup

	NotificationRepo    notificationsapp.Repository
	NotificationService *notificationsapp.Service
	NotificationSender  *notificationsinfra.Sender

	BloodRecipientFinder bloodworkers.BroadcastTargetFinder
}

func NewApp(ctx context.Context) (*App, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}

	log, err := NewLogger(cfg.Log)
	if err != nil {
		return nil, err
	}

	db, err := NewPostgres(ctx, cfg.DB)
	if err != nil {
		return nil, err
	}

	var redisClient *redis.Client
	if cfg.Redis.Addr != "" && cfg.Redis.Addr != "disabled" {
		redisClient, err = NewRedis(ctx, cfg.Redis)
		if err != nil {
			return nil, err
		}
	} else {
		log.Info("Redis disabled, running without cache")
	}

	var bus events.KafkaBus
	if len(cfg.Kafka.Brokers) > 0 && cfg.Kafka.Brokers[0] != "" && cfg.Kafka.Brokers[0] != "disabled" {
		producer, err := NewKafkaSyncProducer(cfg.Kafka)
		if err != nil {
			return nil, err
		}
		bus = events.NewKafkaBus(producer, log)
	} else {
		log.Info("Kafka disabled, using noop event bus")
		bus = &events.NoopBus{}
	}

	// Push/SMS/email delivery happens in-process (no worker, no Kafka hop).
	// A missing or invalid Firebase key disables sending but never blocks boot.
	var pushSender types.NotificationSender
	if sender, senderErr := notificationsinfra.NewSender(
		cfg.Firebase.CredentialsFile,
		notificationsinfra.NewDeviceTokenRepository(db),
	); senderErr != nil {
		log.Warn("push sender disabled: firebase credentials unavailable", zap.Error(senderErr))
	} else {
		pushSender = sender
		log.Info("push sender initialised; notifications deliver in-process")
	}

	r := NewRouter()
	api := r.Group("/api/v1")
	RegisterModules(types.ModuleDeps{
		Router:     api,
		DB:         db,
		Redis:      redisClient,
		Logger:     log,
		Bus:        bus,
		Config:     cfg,
		PushSender: pushSender,
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.App.Port),
		Handler:      r,
		ReadTimeout:  cfg.App.ReadTimeout,
		WriteTimeout: cfg.App.WriteTimeout,
	}

	return &App{cfg: cfg, log: log, http: srv, kafka: bus}, nil
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		a.log.Info("server starting", zap.String("addr", a.http.Addr))
		errCh <- a.http.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.App.ShutdownTimeout)
		defer cancel()
		_ = a.kafka.Close()
		return a.http.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func NewWorker(ctx context.Context) (*Worker, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}

	log, err := NewLogger(cfg.Log)
	if err != nil {
		return nil, err
	}

	db, err := NewPostgres(ctx, cfg.DB)
	if err != nil {
		return nil, err
	}

	producer, err := NewKafkaSyncProducer(cfg.Kafka)
	if err != nil {
		return nil, err
	}

	group, err := NewKafkaConsumerGroup(cfg.Kafka)
	if err != nil {
		return nil, err
	}

	bus := events.NewKafkaBus(producer, log)

	notificationRepo := notificationsinfra.NewRepository(db)
	tokenRepo := notificationsinfra.NewDeviceTokenRepository(db)
	notificationSender, err := notificationsinfra.NewSender(cfg.Firebase.CredentialsFile, tokenRepo)
	if err != nil {
		return nil, err
	}
	notificationService := notificationsapp.NewService(notificationRepo, bus, log, notificationSender)

	bloodRecipientFinder := bloodinfra.NewBroadcastRecipientFinder(db)

	return &Worker{
		Config:               cfg,
		Logger:               log,
		Bus:                  bus,
		Group:                group,
		NotificationRepo:     notificationRepo,
		NotificationService:  notificationService,
		NotificationSender:   notificationSender,
		BloodRecipientFinder: bloodRecipientFinder,
	}, nil
}

func (w *Worker) Run(ctx context.Context) error {
	w.Logger.Info("worker started")
	<-ctx.Done()

	if w.Group != nil {
		_ = w.Group.Close()
	}
	return w.Bus.Close()
}

var _ = gin.H{}

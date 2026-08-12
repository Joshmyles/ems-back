package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

type DeviceTokenResolver interface {
	GetPushTokensByUserID(ctx context.Context, userID string) ([]string, error)
}

type Sender struct {
	fcm      *messaging.Client
	resolver DeviceTokenResolver
}

func NewSender(credentialsFile string, resolver DeviceTokenResolver) (*Sender, error) {
	// The messaging client needs an explicit project ID; read it from the
	// service account file rather than relying on SDK inference.
	raw, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("read firebase credentials: %w", err)
	}
	var creds struct {
		ProjectID   string          `json:"project_id"`
		ProjectInfo json.RawMessage `json:"project_info"`
	}
	if err := json.Unmarshal(raw, &creds); err != nil {
		return nil, fmt.Errorf("parse firebase credentials: %w", err)
	}
	if len(creds.ProjectInfo) > 0 {
		return nil, errors.New("credentials file looks like google-services.json (mobile client config); the backend needs an Admin SDK service account key: Firebase Console > Project settings > Service accounts > Generate new private key")
	}
	if creds.ProjectID == "" {
		return nil, errors.New("firebase credentials file has no project_id")
	}

	app, err := firebase.NewApp(
		context.Background(),
		&firebase.Config{ProjectID: creds.ProjectID},
		option.WithCredentialsFile(credentialsFile),
	)
	if err != nil {
		return nil, fmt.Errorf("init firebase app: %w", err)
	}

	client, err := app.Messaging(context.Background())
	if err != nil {
		return nil, fmt.Errorf("init firebase messaging client: %w", err)
	}

	return &Sender{
		fcm:      client,
		resolver: resolver,
	}, nil
}

func (s *Sender) SendSMS(ctx context.Context, to string, body string) error {
	// integrate SMS provider here
	return nil
}

func (s *Sender) SendEmail(ctx context.Context, to, subject, body string) error {
	// integrate email provider here
	return nil
}

func (s *Sender) SendPush(ctx context.Context, userID string, title, body string) error {
	if s == nil || s.fcm == nil {
		return errors.New("push sender not initialized")
	}
	if s.resolver == nil {
		return errors.New("device token resolver not configured")
	}
	if userID == "" {
		return errors.New("userID is required")
	}

	tokens, err := s.resolver.GetPushTokensByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("resolve device tokens: %w", err)
	}
	if len(tokens) == 0 {
		return nil
	}

	msg := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: map[string]string{
			"user_id": userID,
			"type":    "GENERAL_NOTIFICATION",
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				// Must match the channel created by the mobile app; on
				// Android 8+ the channel owns the sound, "siren" covers older
				// versions (res/raw resource name without extension).
				ChannelID: "emergency",
				Sound:     "siren",
			},
		},
		APNS: &messaging.APNSConfig{
			Headers: map[string]string{
				"apns-priority": "10",
			},
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Sound: "siren.wav",
				},
			},
		},
	}

	resp, err := s.fcm.SendEachForMulticast(ctx, msg)
	if err != nil {
		return fmt.Errorf("send push notification: %w", err)
	}

	if resp.FailureCount == len(tokens) {
		return fmt.Errorf("all push sends failed")
	}

	return nil
}

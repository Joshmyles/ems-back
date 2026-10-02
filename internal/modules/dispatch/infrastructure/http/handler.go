package http

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	dispatchapp "dispatch/internal/modules/dispatch/application"
	dispatchdto "dispatch/internal/modules/dispatch/application/dto"
	platformdb "dispatch/internal/platform/db"
	"dispatch/internal/platform/httpx"
)

type Handler struct{ service *dispatchapp.Service }

func NewHandler(service *dispatchapp.Service) *Handler { return &Handler{service: service} }

// EvaluateAutomaticDispatch godoc
//
//	@Summary		Evaluate automatic dispatch
//	@Description	Uses triage responses to determine whether automatic dispatch should be triggered
//	@Tags			Dispatch
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			payload	body		dispatchdto.EvaluateDispatchRequest	true	"Triage dispatch evaluation payload"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/dispatch/evaluate [post]
func (h *Handler) EvaluateAutomaticDispatch(c *gin.Context) {
	var req dispatchdto.EvaluateDispatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	var actorUserID *string
	if v := c.GetString("user_id"); v != "" {
		actorUserID = &v
	}
	out, err := h.service.EvaluateAutomaticDispatch(c.Request.Context(), req, actorUserID)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// GenerateRecommendations godoc
//
//	@Summary		Generate dispatch recommendations
//	@Description	Generates scored dispatch recommendations for an incident
//	@Tags			Dispatch
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			payload	body		dispatchdto.GenerateRecommendationsRequest	true	"Recommendation payload"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/dispatch/recommendations/generate [post]
func (h *Handler) GenerateRecommendations(c *gin.Context) {
	var req dispatchdto.GenerateRecommendationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	var actorUserID *string
	if v := c.GetString("user_id"); v != "" {
		actorUserID = &v
	}
	out, err := h.service.GenerateRecommendations(c.Request.Context(), req, actorUserID)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// CreateAssignment godoc
//
//	@Summary		Create dispatch assignment
//	@Description	Creates a manual, assisted, or automatic dispatch assignment
//	@Tags			Dispatch
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			payload	body		dispatchdto.CreateDispatchAssignmentRequest	true	"Dispatch assignment payload"
//	@Success		201		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/dispatch/assignments [post]
func (h *Handler) CreateAssignment(c *gin.Context) {
	var req dispatchdto.CreateDispatchAssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.AssignedByUserID == nil {
		if v := c.GetString("user_id"); v != "" {
			req.AssignedByUserID = &v
		}
	}
	out, err := h.service.CreateAssignment(c.Request.Context(), req)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Created(c, out)
}

// UpdateAssignmentStatus godoc
//
//	@Summary		Update dispatch assignment status
//	@Description	Updates dispatch assignment lifecycle status
//	@Tags			Dispatch
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string									true	"Dispatch assignment ID"
//	@Param			payload	body		dispatchdto.UpdateDispatchStatusRequest	true	"Dispatch status payload"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/dispatch/assignments/{id}/status [patch]
func (h *Handler) UpdateAssignmentStatus(c *gin.Context) {
	var req dispatchdto.UpdateDispatchStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	var actorUserID *string
	if v := c.GetString("user_id"); v != "" {
		actorUserID = &v
	}
	out, err := h.service.UpdateAssignmentStatus(c.Request.Context(), c.Param("id"), req, actorUserID)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// GetAssignmentByID godoc
//
//	@Summary		Get dispatch assignment
//	@Description	Returns a dispatch assignment by ID
//	@Tags			Dispatch
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Dispatch assignment ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		500	{object}	map[string]interface{}
//	@Router			/dispatch/assignments/{id} [get]
func (h *Handler) GetAssignmentByID(c *gin.Context) {
	out, err := h.service.GetAssignmentByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// ListAssignments godoc
//
//	@Summary		List dispatch assignments
//	@Description	Returns paginated dispatch assignments
//	@Tags			Dispatch
//	@Produce		json
//	@Security		BearerAuth
//	@Param			incident_id		query		string	false	"Incident ID"
//	@Param			ambulance_id	query		string	false	"Ambulance ID"
//	@Param			status			query		string	false	"Assignment status"
//	@Param			active			query		bool	false	"Only assignments still engaged on a case"
//	@Param			page			query		int		false	"Page number"	default(1)
//	@Param			page_size		query		int		false	"Page size"		default(20)
//	@Param			sort_by			query		string	false	"Sort field"	Enums(created_at,status,assigned_at)
//	@Param			sort_order		query		string	false	"Sort order"	Enums(ASC,DESC)
//	@Success		200				{object}	map[string]interface{}
//	@Failure		500				{object}	map[string]interface{}
//	@Router			/dispatch/assignments [get]
func (h *Handler) ListAssignments(c *gin.Context) {
	var incidentID, ambulanceID, status *string
	if v := c.Query("incident_id"); v != "" {
		incidentID = &v
	}
	if v := c.Query("ambulance_id"); v != "" {
		ambulanceID = &v
	}
	if v := c.Query("status"); v != "" {
		status = &v
	}
	active, _ := strconv.ParseBool(c.Query("active"))
	params := dispatchdto.ListAssignmentsParams{IncidentID: incidentID, AmbulanceID: ambulanceID, Status: status, Active: active,
		Pagination: platformdb.ParsePagination(c.Request.URL.Query(), map[string]string{"created_at": "created_at", "status": "status", "assigned_at": "assigned_at"}, map[string]struct{}{}),
	}
	out, err := h.service.ListAssignments(c.Request.Context(), params)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// ListRecommendations godoc
//
//	@Summary		List dispatch recommendations
//	@Description	Returns generated dispatch recommendations for an incident
//	@Tags			Dispatch
//	@Produce		json
//	@Security		BearerAuth
//	@Param			incident_id	query		string	true	"Incident ID"
//	@Param			page		query		int		false	"Page number"	default(1)
//	@Param			page_size	query		int		false	"Page size"		default(20)
//	@Param			sort_by		query		string	false	"Sort field"	Enums(generated_at,score)
//	@Param			sort_order	query		string	false	"Sort order"	Enums(ASC,DESC)
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	map[string]interface{}
//	@Failure		500			{object}	map[string]interface{}
//	@Router			/dispatch/recommendations [get]
func (h *Handler) ListRecommendations(c *gin.Context) {
	incidentID := strings.TrimSpace(c.Query("incident_id"))
	if incidentID == "" {
		httpx.Error(c, http.StatusBadRequest, "incident_id is required")
		return
	}
	params := dispatchdto.ListRecommendationsParams{IncidentID: incidentID,
		Pagination: platformdb.ParsePagination(c.Request.URL.Query(), map[string]string{"generated_at": "generated_at", "score": "score"}, map[string]struct{}{}),
	}
	out, err := h.service.ListRecommendations(c.Request.Context(), params)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// PersistTriageSession godoc
//
//	@Summary		Persist incident triage
//	@Description	Saves incident triage responses and derives dispatch priority
//	@Tags			Dispatch
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			payload	body		dispatchdto.PersistTriageRequest	true	"Persist triage payload"
//	@Success		201		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/dispatch/triage [post]
func (h *Handler) PersistTriageSession(c *gin.Context) {
	var req dispatchdto.PersistTriageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	var actorUserID *string
	if v := c.GetString("user_id"); v != "" {
		actorUserID = &v
	}
	out, err := h.service.PersistTriageSession(c.Request.Context(), req, actorUserID)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Created(c, out)
}

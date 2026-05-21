package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	facapp "dispatch/internal/modules/facilities/application"
	platformdb "dispatch/internal/platform/db"
	"dispatch/internal/platform/httpx"
)

type Handler struct {
	service *facapp.Service
}

func NewHandler(service *facapp.Service) *Handler {
	return &Handler{service: service}
}

// List godoc
//
//	@Summary		List facilities
//	@Description	Returns paginated facilities with region/district/subcounty hierarchy
//	@Tags			Facilities
//	@Produce		json
//	@Security		BearerAuth
//	@Param			page					query		int		false	"Page number"	default(1)
//	@Param			page_size				query		int		false	"Page size"		default(20)
//	@Param			search					query		string	false	"Search term (facility, district, subcounty, region)"
//	@Param			sort_by					query		string	false	"Sort field"	Enums(facility,level,ownership,region,district,subcounty,facility_uid,subcounty_uid)
//	@Param			sort_order				query		string	false	"Sort order"	Enums(ASC,DESC)
//	@Param			filter[region_uid]		query		string	false	"Filter by region UID"
//	@Param			filter[district_uid]	query		string	false	"Filter by district UID"
//	@Param			filter[subcounty_uid]	query		string	false	"Filter by subcounty UID"
//	@Param			filter[level]			query		string	false	"Filter by facility level"
//	@Param			filter[ownership]		query		string	false	"Filter by ownership"
//	@Success		200						{object}	map[string]interface{}
//	@Failure		500						{object}	map[string]interface{}
//	@Router			/facilities [get]
func (h *Handler) List(c *gin.Context) {
	p := platformdb.ParsePagination(
		c.Request.URL.Query(),
		map[string]string{
			"created_at":    "f.facility", // default sort by facility name
			"facility":      "f.facility",
			"level":         "f.level",
			"ownership":     "f.ownership",
			"region":        "r.region",
			"district":      "d.district",
			"subcounty":     "s.subcounty",
			"facility_uid":  "f.facility_uid",
			"subcounty_uid": "f.subcounty_uid",
		},
		map[string]struct{}{
			"region_uid":    {},
			"district_uid":  {},
			"subcounty_uid": {},
			"level":         {},
			"ownership":     {},
		},
	)
	out, err := h.service.ListFacilities(c.Request.Context(), p)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// Get godoc
//
//	@Summary		Get facility
//	@Description	Get a single facility by UID
//	@Tags			Facilities
//	@Produce		json
//	@Security		BearerAuth
//	@Param			uid	path		string	true	"Facility UID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		404	{object}	map[string]interface{}
//	@Router			/facilities/{uid} [get]
func (h *Handler) Get(c *gin.Context) {
	uid := c.Param("uid")
	out, err := h.service.GetFacility(c.Request.Context(), uid)
	if err != nil {
		httpx.Error(c, http.StatusNotFound, err.Error())
		return
	}
	httpx.OK(c, out)
}

// Create godoc
//
//	@Summary		Create facility
//	@Description	Create a new facility record
//	@Tags			Facilities
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			payload	body		facapp.CreateFacilityRequest	true	"Create facility payload"
//	@Success		201		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/facilities [post]
func (h *Handler) Create(c *gin.Context) {
	var req facapp.CreateFacilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	out, err := h.service.CreateFacility(c.Request.Context(), req)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Created(c, out)
}

// Update godoc
//
//	@Summary		Update facility
//	@Description	Update an existing facility
//	@Tags			Facilities
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			uid		path		string							true	"Facility UID"
//	@Param			payload	body		facapp.UpdateFacilityRequest	true	"Update facility payload"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/facilities/{uid} [put]
func (h *Handler) Update(c *gin.Context) {
	uid := c.Param("uid")
	var req facapp.UpdateFacilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	out, err := h.service.UpdateFacility(c.Request.Context(), uid, req)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// SetFocalPerson godoc
//
//	@Summary		Set facility focal person
//	@Description	Assign or replace the focal person for a facility (one per facility)
//	@Tags			Facilities
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			uid		path		string							true	"Facility UID"
//	@Param			payload	body		facapp.SetFocalPersonRequest	true	"Focal person payload"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		404		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/facilities/{uid}/focal-person [put]
func (h *Handler) SetFocalPerson(c *gin.Context) {
	uid := c.Param("uid")
	var req facapp.SetFocalPersonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	out, err := h.service.SetFocalPerson(c.Request.Context(), uid, req)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// ClearFocalPerson godoc
//
//	@Summary		Clear facility focal person
//	@Description	Remove the focal person from a facility
//	@Tags			Facilities
//	@Produce		json
//	@Security		BearerAuth
//	@Param			uid	path		string	true	"Facility UID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		404	{object}	map[string]interface{}
//	@Failure		500	{object}	map[string]interface{}
//	@Router			/facilities/{uid}/focal-person [delete]
func (h *Handler) ClearFocalPerson(c *gin.Context) {
	uid := c.Param("uid")
	out, err := h.service.ClearFocalPerson(c.Request.Context(), uid)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// Delete godoc
//
//	@Summary		Delete facility
//	@Description	Delete a facility by UID
//	@Tags			Facilities
//	@Produce		json
//	@Security		BearerAuth
//	@Param			uid	path		string	true	"Facility UID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		500	{object}	map[string]interface{}
//	@Router			/facilities/{uid} [delete]
func (h *Handler) Delete(c *gin.Context) {
	uid := c.Param("uid")
	if err := h.service.DeleteFacility(c.Request.Context(), uid); err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, gin.H{"message": "facility deleted"})
}

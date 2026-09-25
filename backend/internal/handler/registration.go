package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wangn-tech/campus-hub/internal/httpx"
	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/service"
	"gorm.io/gorm"
)

type RegistrationHandler struct {
	registrations *service.RegistrationService
	users         *service.UserService
}

func NewRegistrationHandler(registrations *service.RegistrationService, users *service.UserService) *RegistrationHandler {
	return &RegistrationHandler{registrations: registrations, users: users}
}

func (h *RegistrationHandler) Register(c *gin.Context) {
	user, err := currentUser(c, h.users)
	if err != nil {
		userError(c, err)
		return
	}
	view, err := h.registrations.Register(c.Request.Context(), user, c.Param("id"), httpx.TraceID(c))
	if err != nil {
		registrationError(c, err)
		return
	}
	httpx.Success(c, view)
}

func (h *RegistrationHandler) MyRegistrations(c *gin.Context) {
	user, err := currentUser(c, h.users)
	if err != nil {
		userError(c, err)
		return
	}
	page, pageSize := service.NormalizePage(queryInt(c, "page", 0), queryInt(c, "page_size", 0))
	items, total, err := h.registrations.MyRegistrations(c.Request.Context(), user, c.Query("status"), page, pageSize)
	if err != nil {
		registrationError(c, err)
		return
	}
	httpx.Success(c, pageOf(items, page, pageSize, total))
}

func (h *RegistrationHandler) Detail(c *gin.Context) {
	user, err := currentUser(c, h.users)
	if err != nil {
		userError(c, err)
		return
	}
	view, err := h.registrations.Detail(c.Request.Context(), user, c.Param("id"))
	if err != nil {
		registrationError(c, err)
		return
	}
	httpx.Success(c, view)
}

func (h *RegistrationHandler) Cancel(c *gin.Context) {
	user, err := currentUser(c, h.users)
	if err != nil {
		userError(c, err)
		return
	}
	if err := h.registrations.Cancel(c.Request.Context(), user, c.Param("id"), httpx.TraceID(c)); err != nil {
		registrationError(c, err)
		return
	}
	httpx.Success(c, gin.H{"id": c.Param("id"), "status": uint8(model.RegistrationCancelled)})
}

func (h *RegistrationHandler) MyTickets(c *gin.Context) {
	user, err := currentUser(c, h.users)
	if err != nil {
		userError(c, err)
		return
	}
	page, pageSize := service.NormalizePage(queryInt(c, "page", 0), queryInt(c, "page_size", 0))
	items, total, err := h.registrations.MyTickets(c.Request.Context(), user, page, pageSize)
	if err != nil {
		registrationError(c, err)
		return
	}
	httpx.Success(c, pageOf(items, page, pageSize, total))
}

func (h *RegistrationHandler) Ticket(c *gin.Context) {
	user, err := currentUser(c, h.users)
	if err != nil {
		userError(c, err)
		return
	}
	view, err := h.registrations.Ticket(c.Request.Context(), user, c.Param("id"))
	if err != nil {
		registrationError(c, err)
		return
	}
	httpx.Success(c, view)
}

func registrationError(c *gin.Context, e error) {
	switch {
	case errors.Is(e, service.ErrActivityNotFound):
		httpx.Error(c, http.StatusNotFound, 102404, "activity not found")
	case errors.Is(e, service.ErrTicketNotFound):
		httpx.Error(c, http.StatusNotFound, 103404, "ticket not found")
	case errors.Is(e, service.ErrRegistrationNotFound), errors.Is(e, gorm.ErrRecordNotFound):
		httpx.Error(c, http.StatusNotFound, 103404, "registration not found")
	case errors.Is(e, service.ErrForbidden):
		httpx.Error(c, http.StatusForbidden, 103403, "forbidden")
	case errors.Is(e, service.ErrRegistrationNotEligible):
		httpx.Error(c, http.StatusConflict, 103409, "registration requirements not met")
	case errors.Is(e, service.ErrRegistrationConflict):
		httpx.Error(c, http.StatusConflict, 103409, "registration conflict")
	case errors.Is(e, service.ErrStorageUnavailable):
		httpx.Error(c, http.StatusServiceUnavailable, 103503, "file storage unavailable")
	default:
		httpx.Error(c, http.StatusInternalServerError, 103500, "registration request failed")
	}
}

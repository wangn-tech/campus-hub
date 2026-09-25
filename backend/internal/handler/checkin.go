package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wangn-tech/campus-hub/internal/httpx"
	"github.com/wangn-tech/campus-hub/internal/service"
	"gorm.io/gorm"
)

type CheckInHandler struct {
	checkIns *service.CheckInService
	users    *service.UserService
}

func NewCheckInHandler(checkIns *service.CheckInService, users *service.UserService) *CheckInHandler {
	return &CheckInHandler{checkIns: checkIns, users: users}
}

func (h *CheckInHandler) CheckIn(c *gin.Context) {
	var in service.CheckInInput
	if c.ShouldBindJSON(&in) != nil {
		bad(c)
		return
	}
	actor, err := currentUser(c, h.users)
	if err != nil {
		userError(c, err)
		return
	}
	view, err := h.checkIns.CheckIn(c.Request.Context(), actor, in, httpx.TraceID(c))
	if err != nil {
		checkInError(c, err)
		return
	}
	httpx.Success(c, view)
}

func (h *CheckInHandler) List(c *gin.Context) {
	activityID := c.Query("activity_id")
	if activityID == "" {
		checkInError(c, service.ErrCheckInInvalid)
		return
	}
	actor, err := currentUser(c, h.users)
	if err != nil {
		userError(c, err)
		return
	}
	page, pageSize := service.NormalizePage(queryInt(c, "page", 0), queryInt(c, "page_size", 0))
	items, total, err := h.checkIns.List(c.Request.Context(), actor, activityID, page, pageSize)
	if err != nil {
		checkInError(c, err)
		return
	}
	httpx.Success(c, pageOf(items, page, pageSize, total))
}

func checkInError(c *gin.Context, e error) {
	switch {
	case errors.Is(e, service.ErrActivityNotFound):
		httpx.Error(c, http.StatusNotFound, 102404, "activity not found")
	case errors.Is(e, service.ErrTicketNotFound), errors.Is(e, gorm.ErrRecordNotFound):
		httpx.Error(c, http.StatusNotFound, 103404, "ticket not found")
	case errors.Is(e, service.ErrForbidden):
		httpx.Error(c, http.StatusForbidden, 103403, "forbidden")
	case errors.Is(e, service.ErrCheckInInvalid):
		httpx.Error(c, http.StatusBadRequest, 103400, "invalid check-in request")
	case errors.Is(e, service.ErrCheckInConflict):
		httpx.Error(c, http.StatusConflict, 103409, "ticket cannot be checked in")
	default:
		httpx.Error(c, http.StatusInternalServerError, 103500, "check-in request failed")
	}
}

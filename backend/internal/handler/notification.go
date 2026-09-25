package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wangn-tech/campus-hub/internal/httpx"
	"github.com/wangn-tech/campus-hub/internal/service"
)

type NotificationHandler struct {
	notifications *service.NotificationService
	users         *service.UserService
}

func NewNotificationHandler(notifications *service.NotificationService, users *service.UserService) *NotificationHandler {
	return &NotificationHandler{notifications: notifications, users: users}
}

func (h *NotificationHandler) List(c *gin.Context) {
	user, err := currentUser(c, h.users)
	if err != nil {
		userError(c, err)
		return
	}
	page, pageSize := service.NormalizePage(queryInt(c, "page", 0), queryInt(c, "page_size", 0))
	items, total, err := h.notifications.List(c.Request.Context(), user, page, pageSize)
	if err != nil {
		notificationError(c, err)
		return
	}
	httpx.Success(c, pageOf(items, page, pageSize, total))
}

func (h *NotificationHandler) UnreadCount(c *gin.Context) {
	user, err := currentUser(c, h.users)
	if err != nil {
		userError(c, err)
		return
	}
	count, err := h.notifications.UnreadCount(c.Request.Context(), user)
	if err != nil {
		notificationError(c, err)
		return
	}
	httpx.Success(c, gin.H{"count": count})
}

func (h *NotificationHandler) MarkRead(c *gin.Context) {
	var in struct {
		IDs []string `json:"ids"`
	}
	if c.ShouldBindJSON(&in) != nil {
		bad(c)
		return
	}
	user, err := currentUser(c, h.users)
	if err != nil {
		userError(c, err)
		return
	}
	updated, err := h.notifications.MarkRead(c.Request.Context(), user, in.IDs)
	if err != nil {
		notificationError(c, err)
		return
	}
	httpx.Success(c, gin.H{"updated": updated})
}

func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	user, err := currentUser(c, h.users)
	if err != nil {
		userError(c, err)
		return
	}
	updated, err := h.notifications.MarkAllRead(c.Request.Context(), user)
	if err != nil {
		notificationError(c, err)
		return
	}
	httpx.Success(c, gin.H{"updated": updated})
}

func notificationError(c *gin.Context, e error) {
	switch {
	case errors.Is(e, service.ErrNotificationInvalid):
		httpx.Error(c, http.StatusBadRequest, 104400, "invalid notification request")
	case errors.Is(e, service.ErrForbidden):
		httpx.Error(c, http.StatusForbidden, 104403, "forbidden")
	default:
		httpx.Error(c, http.StatusInternalServerError, 104500, "notification request failed")
	}
}

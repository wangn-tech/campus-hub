package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/wangn-tech/campus-hub/internal/httpx"
	"github.com/wangn-tech/campus-hub/internal/service"
)

type ChatHandler struct {
	chats *service.ChatService
	users *service.UserService
}

func NewChatHandler(chats *service.ChatService, users *service.UserService) *ChatHandler {
	return &ChatHandler{chats: chats, users: users}
}

func (h *ChatHandler) MyGroups(c *gin.Context) {
	user, err := currentUser(c, h.users)
	if err != nil {
		userError(c, err)
		return
	}
	items, err := h.chats.MyGroups(c.Request.Context(), user)
	if err != nil {
		chatError(c, err)
		return
	}
	httpx.Success(c, gin.H{"items": items})
}

func (h *ChatHandler) Group(c *gin.Context) {
	user, err := currentUser(c, h.users)
	if err != nil {
		userError(c, err)
		return
	}
	view, err := h.chats.Group(c.Request.Context(), user, c.Param("id"))
	if err != nil {
		chatError(c, err)
		return
	}
	httpx.Success(c, view)
}

func (h *ChatHandler) Members(c *gin.Context) {
	user, err := currentUser(c, h.users)
	if err != nil {
		userError(c, err)
		return
	}
	items, err := h.chats.Members(c.Request.Context(), user, c.Param("id"))
	if err != nil {
		chatError(c, err)
		return
	}
	httpx.Success(c, gin.H{"items": items})
}

func (h *ChatHandler) Messages(c *gin.Context) {
	user, err := currentUser(c, h.users)
	if err != nil {
		userError(c, err)
		return
	}
	items, err := h.chats.Messages(c.Request.Context(), user, c.Param("id"), queryUint64(c, "after_id"), queryInt(c, "limit", 0))
	if err != nil {
		chatError(c, err)
		return
	}
	httpx.Success(c, gin.H{"items": items})
}

func (h *ChatHandler) Offline(c *gin.Context) {
	user, err := currentUser(c, h.users)
	if err != nil {
		userError(c, err)
		return
	}
	items, err := h.chats.Offline(c.Request.Context(), user, queryUint64(c, "after_id"), queryInt(c, "limit", 0))
	if err != nil {
		chatError(c, err)
		return
	}
	httpx.Success(c, gin.H{"items": items})
}

func queryUint64(c *gin.Context, key string) uint64 {
	raw := c.Query(key)
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func chatError(c *gin.Context, e error) {
	switch {
	case errors.Is(e, service.ErrChatGroupNotFound):
		httpx.Error(c, http.StatusNotFound, 104404, "chat group not found")
	case errors.Is(e, service.ErrForbidden):
		httpx.Error(c, http.StatusForbidden, 104403, "forbidden")
	default:
		httpx.Error(c, http.StatusInternalServerError, 104500, "chat request failed")
	}
}

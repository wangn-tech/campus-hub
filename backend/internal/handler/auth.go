package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wangn-tech/campus-hub/internal/httpx"
	"github.com/wangn-tech/campus-hub/internal/service"
	"gorm.io/gorm"
)

type AuthHandler struct{ service *service.AuthService }

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{service: authService}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var input service.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httpx.Error(c, http.StatusBadRequest, 100400, "invalid request body")
		return
	}
	user, err := h.service.Register(c.Request.Context(), input)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			httpx.Error(c, http.StatusConflict, 101409, "email already registered")
			return
		}
		httpx.Error(c, http.StatusBadRequest, 100400, "invalid registration")
		return
	}
	httpx.Success(c, gin.H{"id": user.UUID, "email": user.Email, "nickname": user.Nickname})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var input service.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httpx.Error(c, http.StatusBadRequest, 100400, "invalid request body")
		return
	}
	user, tokens, err := h.service.Login(c.Request.Context(), input)
	if err != nil {
		httpx.Error(c, http.StatusUnauthorized, 101401, "invalid credentials")
		return
	}
	httpx.Success(c, gin.H{"user": gin.H{"id": user.UUID, "email": user.Email, "nickname": user.Nickname}, "tokens": tokens})
}

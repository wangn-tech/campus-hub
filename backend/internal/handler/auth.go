package handler

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/wangn-tech/campus-hub/internal/httpx"
	"github.com/wangn-tech/campus-hub/internal/middleware"
	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/service"
	"net/http"
)

type AuthHandler struct{ service *service.AuthService }

func NewAuthHandler(s *service.AuthService) *AuthHandler { return &AuthHandler{service: s} }
func (h *AuthHandler) Register(c *gin.Context) {
	var in service.RegisterInput
	if c.ShouldBindJSON(&in) != nil {
		bad(c)
		return
	}
	u, e := h.service.Register(c.Request.Context(), in)
	if e != nil {
		authError(c, e)
		return
	}
	httpx.Success(c, gin.H{"id": u.UUID, "email": u.Email, "nickname": u.Nickname})
}
func (h *AuthHandler) Login(c *gin.Context) {
	var in service.LoginInput
	if c.ShouldBindJSON(&in) != nil {
		bad(c)
		return
	}
	u, t, e := h.service.Login(c.Request.Context(), in)
	if e != nil {
		authError(c, e)
		return
	}
	httpx.Success(c, gin.H{"user": gin.H{"id": u.UUID, "email": u.Email, "nickname": u.Nickname}, "tokens": t})
}
func (h *AuthHandler) EmailCode(c *gin.Context) {
	var in struct {
		Email string `json:"email"`
		Scene string `json:"scene"`
	}
	if c.ShouldBindJSON(&in) != nil {
		bad(c)
		return
	}
	if e := h.service.SendEmailCode(c.Request.Context(), in.Email, in.Scene); e != nil {
		authError(c, e)
		return
	}
	httpx.Success(c, gin.H{"status": "sent"})
}
func (h *AuthHandler) Refresh(c *gin.Context) {
	var in service.RefreshInput
	if c.ShouldBindJSON(&in) != nil {
		bad(c)
		return
	}
	t, e := h.service.Refresh(c.Request.Context(), in.RefreshToken)
	if e != nil {
		authError(c, e)
		return
	}
	httpx.Success(c, gin.H{"tokens": t})
}
func (h *AuthHandler) Logout(c *gin.Context) {
	raw, ok := middleware.BearerToken(c)
	if !ok {
		authError(c, service.ErrInvalidCredentials)
		return
	}
	if e := h.service.Logout(c.Request.Context(), raw); e != nil {
		authError(c, e)
		return
	}
	httpx.Success(c, gin.H{"status": "logged_out"})
}
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var in struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if c.ShouldBindJSON(&in) != nil {
		bad(c)
		return
	}
	u := middleware.UserUUID(c)
	user, e := h.user(c, u)
	if e == nil {
		e = h.service.ChangePassword(c.Request.Context(), user, in.OldPassword, in.NewPassword)
	}
	if e != nil {
		authError(c, e)
		return
	}
	httpx.Success(c, gin.H{"status": "password_changed"})
}
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var in struct {
		Email       string `json:"email"`
		Code        string `json:"code"`
		NewPassword string `json:"new_password"`
	}
	if c.ShouldBindJSON(&in) != nil {
		bad(c)
		return
	}
	if e := h.service.ResetPassword(c.Request.Context(), in.Email, in.Code, in.NewPassword); e != nil {
		authError(c, e)
		return
	}
	httpx.Success(c, gin.H{"status": "password_reset"})
}
func (h *AuthHandler) Logoff(c *gin.Context) {
	var in struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if c.ShouldBindJSON(&in) != nil {
		bad(c)
		return
	}
	id := middleware.UserUUID(c)
	u, e := h.user(c, id)
	if e == nil {
		e = h.service.Logoff(c.Request.Context(), u, in.Password, in.Code)
	}
	if e != nil {
		authError(c, e)
		return
	}
	httpx.Success(c, gin.H{"status": "logged_off"})
}
func (h *AuthHandler) user(c *gin.Context, id string) (*model.User, error) {
	return h.service.Current(c.Request.Context(), id)
}
func bad(c *gin.Context) { httpx.Error(c, http.StatusBadRequest, 100400, "invalid request body") }
func authError(c *gin.Context, e error) {
	switch {
	case errors.Is(e, service.ErrRateLimited):
		httpx.Error(c, http.StatusTooManyRequests, 101429, "too many requests")
	case errors.Is(e, service.ErrEmailInUse):
		httpx.Error(c, http.StatusConflict, 101409, "email already registered")
	case errors.Is(e, service.ErrInvalidCode):
		httpx.Error(c, http.StatusBadRequest, 101400, "invalid email code")
	case errors.Is(e, service.ErrInvalidCredentials):
		httpx.Error(c, http.StatusUnauthorized, 101401, "invalid credentials")
	default:
		httpx.Error(c, http.StatusServiceUnavailable, 101503, "authentication unavailable")
	}
}

package handler

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/wangn-tech/campus-hub/internal/httpx"
	"github.com/wangn-tech/campus-hub/internal/middleware"
	"github.com/wangn-tech/campus-hub/internal/service"
	"gorm.io/gorm"
	"net/http"
)

type VerificationHandler struct {
	service *service.VerificationService
	users   *service.UserService
}

func NewVerificationHandler(s *service.VerificationService, u *service.UserService) *VerificationHandler {
	return &VerificationHandler{service: s, users: u}
}
func (h *VerificationHandler) Current(c *gin.Context) {
	u, e := h.users.Current(c.Request.Context(), middleware.UserUUID(c))
	if e != nil {
		userError(c, e)
		return
	}
	v, e := h.service.Current(c.Request.Context(), u)
	if errors.Is(e, gorm.ErrRecordNotFound) {
		httpx.Success(c, nil)
		return
	}
	if e != nil {
		httpx.Error(c, http.StatusInternalServerError, 100500, "load verification failed")
		return
	}
	httpx.Success(c, gin.H{"id": v.UUID, "status": v.Status, "school_name": v.SchoolName, "department": v.Department, "admission_year": v.AdmissionYear})
}
func (h *VerificationHandler) Submit(c *gin.Context) {
	var in service.VerificationInput
	if c.ShouldBindJSON(&in) != nil {
		bad(c)
		return
	}
	u, e := h.users.Current(c.Request.Context(), middleware.UserUUID(c))
	if e == nil {
		v, err := h.service.Submit(c.Request.Context(), u, in, httpx.TraceID(c))
		if err != nil {
			e = err
		} else {
			httpx.Success(c, gin.H{"id": v.UUID, "status": v.Status})
			return
		}
	}
	verificationError(c, e)
}
func (h *VerificationHandler) Confirm(c *gin.Context) {
	u, e := h.users.Current(c.Request.Context(), middleware.UserUUID(c))
	if e == nil {
		e = h.service.Confirm(c.Request.Context(), u, c.Param("id"), httpx.TraceID(c))
	}
	if e != nil {
		verificationError(c, e)
		return
	}
	httpx.Success(c, gin.H{"status": "manual_review"})
}
func (h *VerificationHandler) Cancel(c *gin.Context) {
	var in struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&in)
	u, e := h.users.Current(c.Request.Context(), middleware.UserUUID(c))
	if e == nil {
		e = h.service.Cancel(c.Request.Context(), u, c.Param("id"), in.Reason, httpx.TraceID(c))
	}
	if e != nil {
		verificationError(c, e)
		return
	}
	httpx.Success(c, gin.H{"status": "cancelled"})
}
func verificationError(c *gin.Context, e error) {
	if errors.Is(e, service.ErrForbidden) {
		httpx.Error(c, http.StatusForbidden, 100403, "forbidden")
		return
	}
	httpx.Error(c, http.StatusBadRequest, 101400, "invalid verification request")
}

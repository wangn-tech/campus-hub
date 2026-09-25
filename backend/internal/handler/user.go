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

type UserHandler struct {
	service *service.UserService
	files   *service.FileService
}

func NewUserHandler(s *service.UserService, files ...*service.FileService) *UserHandler {
	h := &UserHandler{service: s}
	if len(files) > 0 {
		h.files = files[0]
	}
	return h
}
func (h *UserHandler) Me(c *gin.Context) {
	u, t, e := h.service.Me(c.Request.Context(), c.MustGet(middleware.UserUUIDKey).(string))
	if e != nil {
		httpx.Error(c, http.StatusUnauthorized, 101401, "authentication required")
		return
	}
	data := gin.H{"id": u.UUID, "email": u.Email, "nickname": u.Nickname, "introduction": u.Introduction, "gender": u.Gender, "birthday": u.Birthday, "interests": t}
	if h.files != nil {
		f, url, err := h.files.Avatar(c.Request.Context(), u)
		if err != nil {
			fileError(c, err)
			return
		}
		if f != nil {
			data["avatar_file_id"] = f.UUID
			data["avatar_url"] = url
		}
	}
	httpx.Success(c, data)
}
func (h *UserHandler) Update(c *gin.Context) {
	var in service.ProfileInput
	if c.ShouldBindJSON(&in) != nil {
		bad(c)
		return
	}
	u, e := h.service.Update(c.Request.Context(), c.MustGet(middleware.UserUUIDKey).(string), in)
	if e != nil {
		userError(c, e)
		return
	}
	data := gin.H{"id": u.UUID, "nickname": u.Nickname, "introduction": u.Introduction, "gender": u.Gender, "birthday": u.Birthday}
	if h.files != nil {
		f, url, err := h.files.Avatar(c.Request.Context(), u)
		if err != nil {
			fileError(c, err)
			return
		}
		if f != nil {
			data["avatar_file_id"] = f.UUID
			data["avatar_url"] = url
		}
	}
	httpx.Success(c, data)
}
func (h *UserHandler) Tags(c *gin.Context) {
	tags, e := h.service.Tags(c.Request.Context())
	if e != nil {
		httpx.Error(c, http.StatusInternalServerError, 100500, "list tags failed")
		return
	}
	httpx.Success(c, gin.H{"items": tags})
}
func (h *UserHandler) Interests(c *gin.Context) {
	var in struct {
		TagIDs []string `json:"tag_ids"`
	}
	if c.ShouldBindJSON(&in) != nil {
		bad(c)
		return
	}
	e := h.service.ReplaceInterests(c.Request.Context(), c.MustGet(middleware.UserUUIDKey).(string), in.TagIDs)
	if e != nil {
		userError(c, e)
		return
	}
	httpx.Success(c, gin.H{"status": "updated"})
}
func userError(c *gin.Context, e error) {
	if errors.Is(e, service.ErrForbidden) {
		httpx.Error(c, http.StatusForbidden, 100403, "forbidden")
		return
	}
	if errors.Is(e, gorm.ErrRecordNotFound) {
		httpx.Error(c, http.StatusBadRequest, 100400, "invalid resource")
		return
	}
	httpx.Error(c, http.StatusInternalServerError, 100500, "request failed")
}

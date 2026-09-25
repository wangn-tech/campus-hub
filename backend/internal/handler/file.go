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

type FileHandler struct {
	files *service.FileService
	users *service.UserService
}

func NewFileHandler(f *service.FileService, u *service.UserService) *FileHandler {
	return &FileHandler{files: f, users: u}
}
func (h *FileHandler) Upload(c *gin.Context) {
	fh, e := c.FormFile("file")
	if e != nil {
		bad(c)
		return
	}
	f, e := fh.Open()
	if e != nil {
		bad(c)
		return
	}
	defer f.Close()
	u, e := h.users.Current(c.Request.Context(), middleware.UserUUID(c))
	if e != nil {
		userError(c, e)
		return
	}
	file, url, e := h.files.UploadImage(c.Request.Context(), u, fh.Filename, c.PostForm("biz_type"), f)
	if e != nil {
		fileError(c, e)
		return
	}
	httpx.Success(c, gin.H{"id": file.UUID, "origin_name": file.OriginName, "mime_type": file.MIMEType, "file_size": file.FileSize, "access_url": url})
}
func (h *FileHandler) Get(c *gin.Context) {
	u, e := h.users.Current(c.Request.Context(), middleware.UserUUID(c))
	if e != nil {
		userError(c, e)
		return
	}
	f, url, e := h.files.Get(c.Request.Context(), u, c.Param("id"))
	if e != nil {
		fileError(c, e)
		return
	}
	httpx.Success(c, gin.H{"id": f.UUID, "origin_name": f.OriginName, "mime_type": f.MIMEType, "file_size": f.FileSize, "access_url": url})
}
func (h *FileHandler) Delete(c *gin.Context) {
	u, e := h.users.Current(c.Request.Context(), middleware.UserUUID(c))
	if e == nil {
		e = h.files.Delete(c.Request.Context(), u, c.Param("id"))
	}
	if e != nil {
		fileError(c, e)
		return
	}
	httpx.Success(c, gin.H{"status": "deleted"})
}
func fileError(c *gin.Context, e error) {
	if errors.Is(e, service.ErrStorageUnavailable) {
		httpx.Error(c, http.StatusServiceUnavailable, 105503, "file storage unavailable")
		return
	}
	if errors.Is(e, service.ErrForbidden) {
		httpx.Error(c, http.StatusForbidden, 100403, "forbidden")
		return
	}
	if errors.Is(e, gorm.ErrRecordNotFound) {
		httpx.Error(c, http.StatusNotFound, 105404, "file not found")
		return
	}
	httpx.Error(c, http.StatusBadRequest, 105400, "invalid file")
}

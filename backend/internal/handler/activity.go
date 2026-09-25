package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/wangn-tech/campus-hub/internal/httpx"
	"github.com/wangn-tech/campus-hub/internal/middleware"
	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/service"
	"gorm.io/gorm"
)

type ActivityHandler struct {
	activities *service.ActivityService
	users      *service.UserService
}

func NewActivityHandler(activities *service.ActivityService, users *service.UserService) *ActivityHandler {
	return &ActivityHandler{activities: activities, users: users}
}

func (h *ActivityHandler) Categories(c *gin.Context) {
	items, err := h.activities.Categories(c.Request.Context())
	if err != nil {
		activityError(c, err)
		return
	}
	httpx.Success(c, gin.H{"items": items})
}

func (h *ActivityHandler) Tags(c *gin.Context) {
	items, err := h.activities.Tags(c.Request.Context(), c.Query("type"))
	if err != nil {
		activityError(c, err)
		return
	}
	httpx.Success(c, gin.H{"items": items})
}

func (h *ActivityHandler) List(c *gin.Context) {
	page, pageSize := service.NormalizePage(queryInt(c, "page", 0), queryInt(c, "page_size", 0))
	query := service.ActivityListQuery{CategoryID: c.Query("category_id"), Sort: c.Query("sort"), Page: page, PageSize: pageSize}
	if raw := c.Query("status"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			bad(c)
			return
		}
		query.Status = &value
	}
	items, total, err := h.activities.List(c.Request.Context(), query)
	if err != nil {
		activityError(c, err)
		return
	}
	httpx.Success(c, pageOf(items, page, pageSize, total))
}

func (h *ActivityHandler) Search(c *gin.Context) {
	page, pageSize := service.NormalizePage(queryInt(c, "page", 0), queryInt(c, "page_size", 0))
	query := service.ActivitySearchQuery{
		Keyword:    c.Query("keyword"),
		CategoryID: c.Query("category_id"),
		TagID:      c.Query("tag_id"),
		Location:   c.Query("location"),
		Page:       page,
		PageSize:   pageSize,
	}
	if value, err := optionalMillis(c, "start_time"); err != nil {
		bad(c)
		return
	} else {
		query.StartTime = value
	}
	if value, err := optionalMillis(c, "end_time"); err != nil {
		bad(c)
		return
	} else {
		query.EndTime = value
	}
	items, total, err := h.activities.Search(c.Request.Context(), query)
	if err != nil {
		activityError(c, err)
		return
	}
	httpx.Success(c, pageOf(items, page, pageSize, total))
}

func (h *ActivityHandler) Detail(c *gin.Context) {
	view, err := h.activities.Detail(c.Request.Context(), middleware.UserUUID(c), c.Param("id"))
	if err != nil {
		activityError(c, err)
		return
	}
	httpx.Success(c, view)
}

func (h *ActivityHandler) MyCreated(c *gin.Context) {
	user, err := h.currentUser(c)
	if err != nil {
		userError(c, err)
		return
	}
	page, pageSize := service.NormalizePage(queryInt(c, "page", 0), queryInt(c, "page_size", 0))
	items, total, err := h.activities.MyCreated(c.Request.Context(), user, page, pageSize)
	if err != nil {
		activityError(c, err)
		return
	}
	httpx.Success(c, pageOf(items, page, pageSize, total))
}

func (h *ActivityHandler) Create(c *gin.Context) {
	var in service.ActivityCreateInput
	if c.ShouldBindJSON(&in) != nil {
		bad(c)
		return
	}
	user, err := h.currentUser(c)
	if err != nil {
		userError(c, err)
		return
	}
	activity, err := h.activities.Create(c.Request.Context(), user, in, httpx.TraceID(c))
	if err != nil {
		activityError(c, err)
		return
	}
	httpx.Success(c, gin.H{"id": activity.UUID, "status": activity.Status})
}

func (h *ActivityHandler) Update(c *gin.Context) {
	var in service.ActivityForm
	if c.ShouldBindJSON(&in) != nil {
		bad(c)
		return
	}
	user, err := h.currentUser(c)
	if err != nil {
		userError(c, err)
		return
	}
	activity, err := h.activities.Update(c.Request.Context(), user, c.Param("id"), in)
	if err != nil {
		activityError(c, err)
		return
	}
	httpx.Success(c, gin.H{"id": activity.UUID, "status": activity.Status})
}

func (h *ActivityHandler) Submit(c *gin.Context) {
	h.transition(c, func(user *model.User) error {
		return h.activities.Submit(c.Request.Context(), user, c.Param("id"), httpx.TraceID(c))
	}, uint8(model.ActivityPendingReview))
}

func (h *ActivityHandler) Cancel(c *gin.Context) {
	h.transition(c, func(user *model.User) error {
		return h.activities.Cancel(c.Request.Context(), user, c.Param("id"), httpx.TraceID(c))
	}, uint8(model.ActivityCancelled))
}

func (h *ActivityHandler) Approve(c *gin.Context) {
	h.transition(c, func(user *model.User) error {
		return h.activities.Approve(c.Request.Context(), user, c.Param("id"), httpx.TraceID(c))
	}, uint8(model.ActivityPublished))
}

func (h *ActivityHandler) SendBack(c *gin.Context) {
	h.transition(c, func(user *model.User) error {
		return h.activities.SendBack(c.Request.Context(), user, c.Param("id"), httpx.TraceID(c))
	}, uint8(model.ActivityDraft))
}

func (h *ActivityHandler) Reject(c *gin.Context) {
	var in struct {
		Reason string `json:"reason"`
	}
	if c.ShouldBindJSON(&in) != nil {
		bad(c)
		return
	}
	h.transition(c, func(user *model.User) error {
		return h.activities.Reject(c.Request.Context(), user, c.Param("id"), in.Reason, httpx.TraceID(c))
	}, uint8(model.ActivityRejected))
}

func (h *ActivityHandler) transition(c *gin.Context, apply func(*model.User) error, status uint8) {
	user, err := h.currentUser(c)
	if err == nil {
		err = apply(user)
	}
	if err != nil {
		activityError(c, err)
		return
	}
	httpx.Success(c, gin.H{"id": c.Param("id"), "status": status})
}

func (h *ActivityHandler) currentUser(c *gin.Context) (*model.User, error) {
	return currentUser(c, h.users)
}

func queryInt(c *gin.Context, key string, fallback int) int {
	raw := c.Query(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func optionalMillis(c *gin.Context, key string) (*int64, error) {
	raw := c.Query(key)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return nil, errors.New("invalid timestamp")
	}
	return &value, nil
}

func activityError(c *gin.Context, e error) {
	switch {
	case errors.Is(e, service.ErrActivityNotFound), errors.Is(e, gorm.ErrRecordNotFound):
		httpx.Error(c, http.StatusNotFound, 102404, "activity not found")
	case errors.Is(e, service.ErrForbidden):
		httpx.Error(c, http.StatusForbidden, 102403, "forbidden")
	case errors.Is(e, service.ErrActivityInvalid):
		httpx.Error(c, http.StatusBadRequest, 102400, "invalid activity request")
	case errors.Is(e, service.ErrActivityConflict):
		httpx.Error(c, http.StatusConflict, 102409, "activity state conflict")
	case errors.Is(e, service.ErrStorageUnavailable):
		httpx.Error(c, http.StatusServiceUnavailable, 102503, "file storage unavailable")
	default:
		httpx.Error(c, http.StatusInternalServerError, 102500, "activity request failed")
	}
}

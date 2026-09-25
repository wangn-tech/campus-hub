package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/wangn-tech/campus-hub/internal/httpx"
	"github.com/wangn-tech/campus-hub/internal/middleware"
	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/service"
)

// pageOf wraps a page of items in the shared pagination envelope.
func pageOf[T any](items []T, page, pageSize int, total int64) httpx.Page[T] {
	return httpx.Page[T]{
		Items:      items,
		Pagination: httpx.Pagination{Page: page, PageSize: pageSize, Total: total},
	}
}

// currentUser resolves the authenticated caller from the request context.
func currentUser(c *gin.Context, users *service.UserService) (*model.User, error) {
	return users.Current(c.Request.Context(), middleware.UserUUID(c))
}

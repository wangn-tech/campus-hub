package httpx

import "github.com/gin-gonic/gin"

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
	TraceID string `json:"trace_id,omitempty"`
}

type Pagination struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

type Page[T any] struct {
	Items      []T        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

func Success(c *gin.Context, data any) {
	c.JSON(200, Response{Code: 0, Message: "success", Data: data, TraceID: TraceID(c)})
}

func Error(c *gin.Context, status int, code int, message string) {
	c.JSON(status, Response{Code: code, Message: message, Data: nil, TraceID: TraceID(c)})
}

func TraceID(c *gin.Context) string {
	value, _ := c.Get("trace_id")
	traceID, _ := value.(string)
	return traceID
}

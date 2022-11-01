package jweb

import (
	"github.com/gin-gonic/gin"
)

type Context interface {
	SetGinContext(ctx *gin.Context)
	GetGinContext() *gin.Context
	ResponseParseParamsFieldFail(path string, field string, value string, err error)
}

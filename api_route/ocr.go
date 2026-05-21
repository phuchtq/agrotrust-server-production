package apiroute

import (
	"raise-child/transport"
	"raise-child/util/middleware"

	"github.com/gin-gonic/gin"
)

func InitialORCRoute(server *gin.Engine) {
	var contextPath string = "ocr"

	var authGroup = server.Group(contextPath, middleware.Authorize)
	authGroup.POST("/upload-child-info", transport.GetExtractChildInfo)
}

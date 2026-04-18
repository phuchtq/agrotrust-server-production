package apiroute

import (
	"raise-child/transport"

	"github.com/gin-gonic/gin"
)

func InitializeCenterRoute(server *gin.Engine) {
	// var contextPath string = "centers"

	// // Rate limiter
	// var listLimit = middleware.InitializeRateLimiter(rate.Every(time.Second/2), 20)

	// // Normal group
	// var norGroup = server.Group(contextPath)
	// norGroup.GET("", middleware.RateLimitMiddleware(listLimit), transport.GetCenters)

	var contextPath string = "centers"

	// Normal group
	var norGroup = server.Group(contextPath)
	norGroup.GET("", transport.GetCenters)
}

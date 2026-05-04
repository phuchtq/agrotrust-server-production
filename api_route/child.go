package apiroute

import (
	"raise-child/transport"
	"raise-child/util/middleware"

	"github.com/gin-gonic/gin"
)

// func InitializeChildRoutes(server *gin.Engine) {
// 	var contextPath string = "children"

// 	// Normal group
// 	var norGroup = server.Group(contextPath)
// 	norGroup.GET("", transport.GetChildren)
// 	norGroup.GET("/:id", transport.GetChild)

// 	// Auth group
// 	var authGroup = server.Group(contextPath, middleware.Authorize)
// 	authGroup.PUT("/Metadata/string/:id", transport.AddChildStringMetadata)
// 	authGroup.PUT("/Metadata/number/:id", transport.AddChildNumberMetadata)
// 	authGroup.POST("", transport.UploadChild)
// }

func InitializeChildRoutes(server *gin.Engine) {
	// var contextPath string = "children"

	// // Rate limits
	// var listLimit = middleware.InitializeRateLimiter(rate.Every(time.Second/2), 10)
	// var detailLimit = middleware.InitializeRateLimiter(rate.Every(time.Second/5), 20)
	// var uploadLimit = middleware.InitializeRateLimiter(rate.Every(time.Minute/30), 30)
	// var MetadataLimit = middleware.InitializeRateLimiter(rate.Every(time.Second/1), 10)
	// var donateLimit = middleware.InitializeRateLimiter(rate.Every(time.Minute/4), 7)
	// var proposalLimit = middleware.InitializeRateLimiter(rate.Every(time.Minute/3), 5)
	// var voteLimit = middleware.InitializeRateLimiter(rate.Every(time.Second/1), 5)
	// var confirmLimit = middleware.InitializeRateLimiter(rate.Every(time.Minute/2), 5)
	// var editNeedLimit = middleware.InitializeRateLimiter(rate.Every(time.Minute/6), 10)

	// // Normal group
	// var norGroup = server.Group(contextPath)
	// norGroup.GET("", middleware.RateLimitMiddleware(listLimit), transport.GetChildren)
	// norGroup.GET("/:id", middleware.RateLimitMiddleware(detailLimit), transport.GetChild)

	// // Auth group
	// var authGroup = server.Group(contextPath, middleware.Authorize)
	// authGroup.PUT("/Metadata/string/:id", middleware.RateLimitMiddleware(MetadataLimit), transport.AddChildStringMetadata)
	// authGroup.PUT("/Metadata/number/:id", middleware.RateLimitMiddleware(MetadataLimit), transport.AddChildNumberMetadata)
	// authGroup.POST("", middleware.RateLimitMiddleware(uploadLimit), transport.UploadChild)
	// authGroup.POST("/books-need/:id/support", middleware.RateLimitMiddleware(donateLimit), transport.SupportBooksNeed)
	// authGroup.POST("/health-insurance-need/:id/support", middleware.RateLimitMiddleware(donateLimit), transport.SupportHealthInsuranceNeed)
	// authGroup.POST("/meal-need/:id/support", middleware.RateLimitMiddleware(donateLimit), transport.SupportMealNeed)
	// authGroup.POST("/special-need/:id/support", middleware.RateLimitMiddleware(donateLimit), transport.SupportSpecialNeed)
	// authGroup.POST("/special-need/proposal/:id/vote", middleware.RateLimitMiddleware(voteLimit), transport.VoteSpecialNeedProposal)

	// // Manager group
	// var managerGroup = server.Group(contextPath, middleware.Authorize, middleware.ManagerRoleAuthorize)
	// managerGroup.POST("/books-need/withdraw-proposal", middleware.RateLimitMiddleware(proposalLimit), transport.CreateBooksNeedWithdrawProposal)
	// managerGroup.POST("/health-insurance-need/withdraw-proposal", middleware.RateLimitMiddleware(proposalLimit), transport.CreateHealthInsuranceNeedWithdrawProposal)
	// managerGroup.POST("/meal-need/withdraw-proposal", middleware.RateLimitMiddleware(proposalLimit), transport.CreateMealNeedWithdrawProposal)
	// managerGroup.POST("/special-need/withdraw-proposal", middleware.RateLimitMiddleware(proposalLimit), transport.CreateSpecialNeedWithdrawProposal)
	// managerGroup.POST("/special-need/proposal", middleware.RateLimitMiddleware(proposalLimit), transport.CreateSpecialNeedProposal)
	// managerGroup.POST("/special-need/proposal/:id/confirm", middleware.RateLimitMiddleware(proposalLimit), transport.ConfirmSpecialNeedProposal)

	// // Staff group
	// var staffGroup = server.Group(contextPath, middleware.Authorize, middleware.StaffRoleAuthorize)
	// staffGroup.POST("/:id/provide-meal/confirm", middleware.RateLimitMiddleware(confirmLimit), transport.ConfirmProvideMealForChild)

	// // Leader group
	// var leaderGroup = server.Group(contextPath, middleware.Authorize, middleware.LeaderAuthorize)
	// leaderGroup.PUT("/books-need", middleware.RateLimitMiddleware(editNeedLimit), transport.UpdateBooksNeed)
	// leaderGroup.PUT("/meal-need", middleware.RateLimitMiddleware(editNeedLimit), transport.UpdateMealNeed)
	// leaderGroup.PUT("/health-insurance-need", middleware.RateLimitMiddleware(editNeedLimit), transport.UpdateHealthInsuranceNeed)

	var contextPath string = "children"

	// Normal group
	var norGroup = server.Group(contextPath)
	norGroup.GET("", transport.GetChildren)
	norGroup.GET("/user/:id/supported", transport.GetUserSupportedChildren)
	norGroup.GET("/:id", transport.GetChild)

	// Auth group
	var authGroup = server.Group(contextPath, middleware.Authorize)
	authGroup.PUT("/Metadata/string/:id", transport.AddChildStringMetadata)
	authGroup.PUT("/Metadata/number/:id", transport.AddChildNumberMetadata)
	authGroup.POST("", transport.UploadChild)
	authGroup.POST("/books-need/:id/support", transport.SupportBooksNeed)
	authGroup.POST("/health-insurance-need/:id/support", transport.SupportHealthInsuranceNeed)
	authGroup.POST("/meal-need/:id/support", transport.SupportMealNeed)
	authGroup.POST("/special-need/:id/support", transport.SupportSpecialNeed)
	authGroup.POST("/special-need/proposal/:id/vote", transport.VoteSpecialNeedProposal)

	// Manager group
	var managerGroup = server.Group(contextPath, middleware.Authorize, middleware.ManagerRoleAuthorize)
	managerGroup.POST("/books-need/withdraw-proposal", transport.CreateBooksNeedWithdrawProposal)
	managerGroup.POST("/health-insurance-need/withdraw-proposal", transport.CreateHealthInsuranceNeedWithdrawProposal)
	managerGroup.POST("/meal-need/withdraw-proposal", transport.CreateMealNeedWithdrawProposal)
	managerGroup.POST("/special-need/withdraw-proposal", transport.CreateSpecialNeedWithdrawProposal)
	managerGroup.POST("/special-need/proposal", transport.CreateSpecialNeedProposal)
	managerGroup.POST("/special-need/proposal/:id/confirm", transport.ConfirmSpecialNeedProposal)

	// Staff group
	var staffGroup = server.Group(contextPath, middleware.Authorize, middleware.StaffRoleAuthorize)
	staffGroup.POST("/:id/provide-meal/confirm", transport.ConfirmProvideMealForChild)

	// Leader group
	var leaderGroup = server.Group(contextPath, middleware.Authorize, middleware.LeaderAuthorize)
	leaderGroup.PUT("/books-need", transport.UpdateBooksNeed)
	leaderGroup.PUT("/meal-need", transport.UpdateMealNeed)
	leaderGroup.PUT("/health-insurance-need", transport.UpdateHealthInsuranceNeed)
}

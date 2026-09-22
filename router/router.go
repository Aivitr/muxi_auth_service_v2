package router

import (
	"net/http"

	"github.com/Muxi-X/muxi_auth_service_v2/handler/check"
	"github.com/Muxi-X/muxi_auth_service_v2/handler/email"
	"github.com/Muxi-X/muxi_auth_service_v2/handler/member"
	"github.com/Muxi-X/muxi_auth_service_v2/handler/oauth"
	"github.com/Muxi-X/muxi_auth_service_v2/handler/password"
	"github.com/Muxi-X/muxi_auth_service_v2/handler/sd"
	"github.com/Muxi-X/muxi_auth_service_v2/handler/signin"
	"github.com/Muxi-X/muxi_auth_service_v2/handler/signup"
	"github.com/Muxi-X/muxi_auth_service_v2/handler/user"
	"github.com/Muxi-X/muxi_auth_service_v2/router/middleware"

	"github.com/gin-gonic/gin"
)

// Load loads the middlewares, routes, handlers.
func Load(g *gin.Engine, mw ...gin.HandlerFunc) *gin.Engine {
	// Middlewares.
	g.Use(gin.Recovery())
	g.Use(middleware.NoCache)
	g.Use(middleware.Options)
	g.Use(middleware.Secure)

	g.Use(mw...)
	// 404 Handler.
	g.NoRoute(func(c *gin.Context) {
		c.String(http.StatusNotFound, "The incorrect API route.")
	})

	authRouter := g.Group("/auth/api")
	{
		authRouter.POST("/signup", signup.UserSignup)
		authRouter.POST("/signin", signin.UserSignin)
		authRouter.GET("/check_name", check.CheckUsernameExisted)
		authRouter.GET("/check_email", check.CheckEmailExisted)
		authRouter.GET("/check_token", check.CheckToken)
		authRouter.GET("/email", email.GetEmailByUsername)
		authRouter.POST("/password/get_captcha", password.GetCaptcha)
		authRouter.POST("/password/check_captcha", password.CheckCaptcha)
		authRouter.POST("/password/reset", password.PasswordReset)

		authRouter.POST("/oauth", oauth.Auth)
		authRouter.GET("/oauth/cas/callback", oauth.CASCallback)
		authRouter.POST("/oauth/token", oauth.Token)
		authRouter.POST("/oauth/token/refresh", oauth.Refresh)
		authRouter.POST("/oauth/store", middleware.AdminRequiredMiddleware(), oauth.AdminStore)
	}

	userRouter := g.Group("/auth/api/user")
	userRouter.Use(middleware.LoginRequiredMiddleware())
	{
		userRouter.GET("", user.Get)
	}

	adminRouter := g.Group("/auth/api/admin")
	adminRouter.Use(middleware.AdminRequiredMiddleware())
	{
		adminRouter.POST("/oauth/store", oauth.AdminStore)

		adminRouter.GET("/members", member.List)
		adminRouter.POST("/members", member.Create)
		adminRouter.PUT("/members/:user_id", member.Update)
		adminRouter.DELETE("/members/:user_id", member.Delete)
		adminRouter.GET("/users/search", member.SearchUsers)
	}

	// The health check handlers
	svcd := g.Group("/sd")
	{
		svcd.GET("/health", sd.HealthCheck)
		svcd.GET("/disk", sd.DiskCheck)
		svcd.GET("/cpu", sd.CPUCheck)
		svcd.GET("/ram", sd.RAMCheck)
	}

	return g
}

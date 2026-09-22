package middleware

import (
	"github.com/Muxi-X/muxi_auth_service_v2/handler"
	"github.com/Muxi-X/muxi_auth_service_v2/model"
	"github.com/Muxi-X/muxi_auth_service_v2/pkg/errno"
	"github.com/Muxi-X/muxi_auth_service_v2/pkg/oauth"

	"github.com/gin-gonic/gin"
)

func LoginRequiredMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, err := oauth.ParseRequest(c)
		if err != nil {
			handler.SendUnauthorized(c, errno.ErrTokenInvalid, nil, err.Error())
			c.Abort()
			return
		}
		c.Set("principal", principal)
		if principal.LocalUserID != 0 {
			c.Set("userID", principal.LocalUserID)
		}
		if principal.CASUsername != "" {
			c.Set("casUsername", principal.CASUsername)
		}

		c.Next()
	}
}

func AdminRequiredMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, err := oauth.ParseRequest(c)
		if err != nil {
			handler.SendUnauthorized(c, errno.ErrTokenInvalid, nil, err.Error())
			c.Abort()
			return
		}
		if principal.LocalUserID == 0 {
			handler.SendForbidden(c, errno.ErrPermissionDenied, nil, "admin api only accepts local user access tokens")
			c.Abort()
			return
		}

		user, err := model.GetUserByID(principal.LocalUserID)
		if err != nil || !model.HasAdminAccess(user) {
			handler.SendForbidden(c, errno.ErrPermissionDenied, nil, "admin permission is required")
			c.Abort()
			return
		}

		c.Set("principal", principal)
		c.Set("userID", principal.LocalUserID)
		c.Set("adminUser", user)
		c.Next()
	}
}

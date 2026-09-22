package check

import (
	"github.com/Muxi-X/muxi_auth_service_v2/handler"
	"github.com/Muxi-X/muxi_auth_service_v2/model"
	"github.com/Muxi-X/muxi_auth_service_v2/pkg/auth"
	"github.com/Muxi-X/muxi_auth_service_v2/pkg/errno"
	"github.com/gin-gonic/gin"
)

func CheckToken(c *gin.Context) {
	var token, email string
	var ok bool
	if token, ok = c.GetQuery("token"); !ok {
		handler.SendBadRequest(c, errno.ErrBadRequest, nil, "Token is required.")
		return
	}
	if email, ok = c.GetQuery("email"); !ok {
		handler.SendBadRequest(c, errno.ErrBadRequest, nil, "Email is required.")
		return
	}

	tokenResolve, err := auth.ResolveToken(token)
	if err != nil {
		// 不要在这里打印 token/email：本端点公开无鉴权，任何人都能拿一个坏 token
		// 把当次请求的凭据推进日志，而日志的留存期比 token 的 7 天有效期还长。
		handler.SendError(c, errno.ErrTokenInvalid, nil, err.Error())
		return
	}

	user, err := model.GetUserByEmail(email)
	if err != nil {
		handler.SendNotFound(c, errno.ErrUserNotFound, nil, err.Error())
		return
	}

	if tokenResolve.ID != user.Id {
		handler.SendError(c, errno.ErrTokenInvalid, nil, "ID not match.")
		return
	}

	handler.SendResponse(c, nil, "OK")
	return
}

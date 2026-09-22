package user

import (
	"github.com/Muxi-X/muxi_auth_service_v2/handler"
	"github.com/Muxi-X/muxi_auth_service_v2/model"
	"github.com/Muxi-X/muxi_auth_service_v2/pkg/oauth"

	"github.com/gin-gonic/gin"
)

func Get(c *gin.Context) {
	principal := c.MustGet("principal").(oauth.AccessPrincipal)

	userID := principal.LocalUserID
	// 兼容旧的 cas:<username> access token（新签发的已统一为本地 user id）。
	// 反查一下本地账号，让这批历史 token 也能拿到正确的 is_muxi_member。
	if userID == 0 && principal.CASUsername != "" {
		if identity, err := model.GetUserIdentity("cas", principal.CASUsername); err == nil {
			userID = identity.UserID
		}
	}

	if userID == 0 {
		handler.SendResponse(c, nil, oauth.BuildCASUserInfo(principal.CASUsername))
		return
	}

	user, err := model.GetUserInfoByID(userID)
	if err != nil {
		handler.SendError(c, err, nil, err.Error())
		return
	}

	handler.SendResponse(c, nil, user)
}

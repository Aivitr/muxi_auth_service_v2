package oauth

import (
	"github.com/Muxi-X/muxi_auth_service_v2/handler"
	"github.com/Muxi-X/muxi_auth_service_v2/pkg/errno"
	"github.com/Muxi-X/muxi_auth_service_v2/pkg/logx"
	. "github.com/Muxi-X/muxi_auth_service_v2/pkg/oauth"

	"github.com/gin-gonic/gin"
	"gopkg.in/oauth2.v4"
	e "gopkg.in/oauth2.v4/errors"
)

// 更新 access token
// Params:
//
//	grant_type: refresh_token
//	client_id:
//
// Forms:
//
//	client_secret:
//	refresh_token:
func Refresh(c *gin.Context) {
	grantType, ok := c.GetQuery("grant_type")
	if !ok || grantType != "refresh_token" {
		handler.SendBadRequest(c, errno.ErrBadRequest, nil, "grant_type is required and must be refresh_token")
		return
	}

	refreshToken, ok := c.GetPostForm("refresh_token")
	if !ok {
		handler.SendBadRequest(c, errno.ErrBadRequest, nil, "refresh_token is required")
		return
	}

	clientID, clientSecret, err := OauthServer.Server.ClientInfoHandler(c.Request)
	if err != nil {
		handler.SendBadRequest(c, errno.ErrBadRequest, nil, err.Error())
		return
	}

	// 禁止传 Scope：客户端塞一个 scope=muxi:member 就能自己给自己提权，
	// 绕过 CheckMuxiMemberScope。另一道保险见 pkg/oauth/server.go。
	tgr := &oauth2.TokenGenerateRequest{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Request:      c.Request,
		Refresh:      refreshToken,
	}

	tokenInfo, err := OauthServer.Server.GetAccessToken(c, oauth2.GrantType(grantType), tgr)
	if err != nil {
		// 刷新 token 失败时记录结构化日志，便于区分是 refresh token 过期还是服务端内部错误。
		logx.Error("GetAccessToken error", "error", err)
		errCase := err.Error()
		if err == e.ErrInvalidGrant {
			errCase = "The refresh token is invalid or has expired"
		}
		handler.SendError(c, errno.ErrRefreshToken, nil, errCase)
		return
	}

	handler.SendResponse(c, nil, AccessTokenResponse{
		AccessToken:    tokenInfo.GetAccess(),
		AccessExpired:  int64(tokenInfo.GetAccessExpiresIn().Seconds()),
		RefreshToken:   tokenInfo.GetRefresh(),
		RefreshExpired: int64(tokenInfo.GetRefreshExpiresIn().Seconds()),
	})
}

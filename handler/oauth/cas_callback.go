package oauth

import (
	"errors"
	"net/http"

	"github.com/Muxi-X/muxi_auth_service_v2/handler"
	"github.com/Muxi-X/muxi_auth_service_v2/pkg/errno"
	"github.com/Muxi-X/muxi_auth_service_v2/service"

	"github.com/gin-gonic/gin"
)

// casOAuthFlowFactory 允许测试按需替换 callback 的核心流程实现。
// 正常运行时这里始终返回生产环境下的默认 CAS -> OAuth 编排器。
var casOAuthFlowFactory = service.NewDefaultCASOAuthFlow

// CASCallback 负责处理 CAS 登录成功后的回调，并将其翻译为现有 OAuth 授权码。
// Handler 本身只做参数入口和 HTTP 响应，具体业务流程全部委托给 service 层。
func CASCallback(c *gin.Context) {
	flow, err := casOAuthFlowFactory()
	if err != nil {
		handler.SendError(c, errno.ErrGenerateAuthCode, nil, err.Error())
		return
	}

	result, err := flow.HandleCallback(c, c.Request)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMissingCASTicket),
			errors.Is(err, service.ErrMissingOAuthClientID),
			errors.Is(err, service.ErrMissingCallbackURL):
			handler.SendBadRequest(c, errno.ErrBadRequest, nil, err.Error())
			return
		case errors.Is(err, service.ErrInvalidCASTicket):
			handler.SendUnauthorized(c, errno.ErrInvalidCASTicket, nil, err.Error())
			return
		case errors.Is(err, errno.ErrNotMuxiMember):
			handler.SendForbidden(c, errno.ErrNotMuxiMember, nil, "muxi:member scope requires a muxi member account")
			return
		default:
			handler.SendError(c, errno.ErrGenerateAuthCode, nil, err.Error())
			return
		}
	}

	c.Redirect(http.StatusFound, result.RedirectURL)
}

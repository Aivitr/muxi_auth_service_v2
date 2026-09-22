package middleware

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// sensitiveQueryKeys 是不该进 access log 的 query 参数名。
// gin 默认的 formatter 会把 Path 连同 RawQuery 整条打进日志，
// 而 /auth/api/check_token 的 token 是 7 天有效的登录凭据，
// 容器部署时 stdout 就是被收集走的日志。
var sensitiveQueryKeys = map[string]bool{
	"token":         true,
	"password":      true,
	"code":          true,
	"refresh_token": true,
	"client_secret": true,
	"captcha":       true,
	"ticket":        true,
}

// SensitiveLogFormatter 给 gin.LoggerWithFormatter 用。
// 与 gin 默认格式的唯一区别是把敏感 query 的值换成 ***。
func SensitiveLogFormatter(param gin.LogFormatterParams) string {
	var statusColor, methodColor, resetColor string
	if param.IsOutputColor() {
		statusColor = param.StatusCodeColor()
		methodColor = param.MethodColor()
		resetColor = param.ResetColor()
	}

	if param.Latency > time.Minute {
		param.Latency = param.Latency - param.Latency%time.Second
	}

	return fmt.Sprintf("[GIN] %v |%s %3d %s| %13v | %15s |%s %-7s %s %#v\n%s",
		param.TimeStamp.Format("2006/01/02 - 15:04:05"),
		statusColor, param.StatusCode, resetColor,
		param.Latency,
		param.ClientIP,
		methodColor, param.Method, resetColor,
		redactQuery(param.Path),
		param.ErrorMessage,
	)
}

// redactQuery 逐段替换敏感参数的值，不重新做 URL 编码：
// 重编码会把 *** 转成 %2A%2A%2A，日志反而更难读。
func redactQuery(path string) string {
	i := strings.IndexByte(path, '?')
	if i < 0 {
		return path
	}

	parts := strings.Split(path[i+1:], "&")
	for j, part := range parts {
		key, _, found := strings.Cut(part, "=")
		if found && sensitiveQueryKeys[strings.ToLower(key)] {
			parts[j] = key + "=***"
		}
	}

	return path[:i] + "?" + strings.Join(parts, "&")
}

package oauth

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Muxi-X/muxi_auth_service_v2/model"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

const casSubjectPrefix = "cas:"

// AccessPrincipal 表示从 access token 中解析出来的认证主体。
// 新签发的本地登录与 CAS 登录 token 都会解析成本地 users.id；
// cas:<username> 仅用于兼容历史 CAS token。
type AccessPrincipal struct {
	Subject     string
	LocalUserID uint64
	CASUsername string
}

// BuildCASSubject 会把 CAS 用户名编码成历史 CAS subject。
// 新 token 已统一使用本地 users.id；该格式仅用于兼容旧 token。
func BuildCASSubject(casUsername string) string {
	return casSubjectPrefix + casUsername
}

func ResolveAccessToken(accessToken string) (string, error) {
	token, err := jwt.Parse(accessToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtKey), nil
	})

	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		sub, ok := claims["sub"]
		if !ok {
			return "", errors.New("Token not include `cap` field.")
		}
		return sub.(string), nil
	}
	return "", errors.New("Unknown error.")
}

// ResolvePrincipalFromSubject 负责把 subject 字符串解析为统一主体信息。
// 这个函数拆出来是为了让测试和业务代码都能直接验证 subject 语义，
// 不必每次都先伪造一整个 JWT。
func ResolvePrincipalFromSubject(subject string) (AccessPrincipal, error) {
	if strings.HasPrefix(subject, casSubjectPrefix) {
		casUsername := strings.TrimPrefix(subject, casSubjectPrefix)
		if casUsername == "" {
			return AccessPrincipal{}, errors.New("cas subject is empty")
		}

		return AccessPrincipal{
			Subject:     subject,
			CASUsername: casUsername,
		}, nil
	}

	localUserID, err := strconv.ParseUint(subject, 10, 64)
	if err != nil {
		return AccessPrincipal{}, err
	}

	return AccessPrincipal{
		Subject:     subject,
		LocalUserID: localUserID,
	}, nil
}

// ResolvePrincipalFromToken 会把 token subject 解析为统一主体信息。
// 数字 subject 视为本地用户，cas: 前缀仅兼容历史 CAS token。
func ResolvePrincipalFromToken(token string) (AccessPrincipal, error) {
	subject, err := ResolveAccessToken(token)
	if err != nil {
		return AccessPrincipal{}, err
	}
	return ResolvePrincipalFromSubject(subject)
}

// BuildCASUserInfo 只为反查不到本地账号的历史 CAS 主体兜底，一律按非成员返回。
// Roles 必须显式给非 nil 切片。
func BuildCASUserInfo(casUsername string) *model.UserInfo {
	return &model.UserInfo{
		Username: casUsername,
		Info:     "cas authenticated user",
		Roles:    []string{model.RoleNameUser},
	}
}

func ParseRequest(c *gin.Context) (AccessPrincipal, error) {
	token := c.GetHeader("token")
	if token == "" {
		return AccessPrincipal{}, errors.New("token is required")
	}
	return ResolvePrincipalFromToken(token)
}

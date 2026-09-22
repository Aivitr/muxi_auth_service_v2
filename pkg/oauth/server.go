package oauth

import (
	"net/http"
	"time"

	"github.com/Muxi-X/muxi_auth_service_v2/pkg/logx"
	store "github.com/Shadowmaple/oauth2-mysql-store"
	"github.com/dgrijalva/jwt-go"
	"github.com/spf13/viper"
	"gopkg.in/oauth2.v4"
	"gopkg.in/oauth2.v4/errors"
	"gopkg.in/oauth2.v4/generates"
	"gopkg.in/oauth2.v4/manage"
	"gopkg.in/oauth2.v4/server"
)

const (
	authCodeExp     = time.Minute * 30
	accessTokenExp  = time.Hour * 24 * 30
	refreshTokenExp = time.Hour * 24 * 365 * 5

	tokenStoreTableName  = "oauth2_token"
	clientStoreTableName = "oauth2_client"
)

type OauthServerModel struct {
	Server      *server.Server
	ClientStore *store.ClientStore
}

var (
	OauthServer *OauthServerModel
	jwtKey      string
)

func (*OauthServerModel) Init() {
	clientStore := getClientStore()
	manager := getManager()
	manager.MapClientStorage(clientStore)

	srv := server.NewDefaultServer(manager)

	serverConfig(srv)

	OauthServer = &OauthServerModel{
		Server:      srv,
		ClientStore: clientStore,
	}
}

func serverConfig(srv *server.Server) {
	srv.SetAllowedGrantType(oauth2.AuthorizationCode, oauth2.Refreshing)
	srv.SetAllowGetAccessRequest(false)

	srv.SetInternalErrorHandler(func(err error) (re *errors.Response) {
		// 统一记录 OAuth 服务内部错误，避免日志格式在升级后继续分裂。
		logx.Infof("Internal Error: %s", err.Error())
		return
	})

	srv.SetResponseErrorHandler(func(re *errors.Response) {
		if re == nil || re.Error == nil {
			logx.Info("Response Error: empty oauth response error")
			return
		}

		// 这里记录 OAuth 协议层面的响应错误，便于区分业务报错与框架报错。
		logx.Infof("Response Error: %s", re.Error.Error())
	})

	// UserAuthorizationHandler get user id from request authorization
	srv.SetUserAuthorizationHandler(func(w http.ResponseWriter, r *http.Request) (userID string, err error) {
		// return "", errors.ErrAccessDenied
		// userID = r.Context().Value("userID").(string)
		return
	})

	// get client info (clientID and clientSecret)
	srv.SetClientInfoHandler(func(r *http.Request) (clientID, clientSecret string, err error) {
		clientID = r.FormValue("client_id")
		clientSecret = r.FormValue("client_secret")
		// 这里不再打印 client_secret，避免敏感信息继续进入日志系统。
		logx.Info("client info parsed", "client_id", clientID)
		return
	})

	// ClientAuthorizedHandler check the client allows to use this authorization grant type
	srv.SetClientAuthorizedHandler(func(clientID string, grant oauth2.GrantType) (allowed bool, err error) {
		return true, nil
	})

	// RefreshingScopeHandler 禁止在 refresh 时改动 scope：谁能让 refresh 带上 scope，
	// 谁就能给自己的 token 加 muxi:member，绕过 CheckMuxiMemberScope。
	//
	// 今天这条路径是死的（handler/oauth/refresh.go 没给 tgr 设 Scope），
	// 但那是「一行代码之差」的安全边界，真有人补上 Scope 时这里一律 false 兜住。
	srv.SetRefreshingScopeHandler(func(newScope, oldScope string) (allowed bool, err error) {
		return false, nil
	})
}

func getManager() *manage.Manager {
	manager := manage.NewDefaultManager()

	manager.SetAuthorizeCodeTokenCfg(&manage.Config{
		AccessTokenExp:    accessTokenExp,
		RefreshTokenExp:   refreshTokenExp,
		IsGenerateRefresh: true,
	})

	manager.SetAuthorizeCodeExp(authCodeExp)

	// token store
	manager.MapTokenStorage(getTokenStore())
	// token generate
	jwtKey = viper.GetString("jwt_secret")
	manager.MapAccessGenerate(generates.NewJWTAccessGenerate([]byte(jwtKey), jwt.SigningMethodHS512))

	// client store
	clientStore := getClientStore()
	manager.MapClientStorage(clientStore)

	return manager
}

func getTokenStore() oauth2.TokenStore {
	return store.NewTokenStore(&store.TokenConfig{
		BasicConfig: getDBBasicConfig(tokenStoreTableName),
		GcDisabled:  false,
		GcInterval:  time.Hour * 2,
	})
}

func getClientStore() *store.ClientStore {
	return store.NewClientStore(&store.ClientConfig{
		BasicConfig: getDBBasicConfig(clientStoreTableName),
	})
}

func getDBBasicConfig(table string) store.BasicConfig {
	return store.BasicConfig{
		Addr:     viper.GetString("db.addr"),
		UserName: viper.GetString("db.username"),
		Password: viper.GetString("db.password"),
		Database: viper.GetString("db.name"),
		Table:    table,
	}
}

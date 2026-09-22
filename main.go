package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/Muxi-X/muxi_auth_service_v2/config"
	"github.com/Muxi-X/muxi_auth_service_v2/model"
	"github.com/Muxi-X/muxi_auth_service_v2/pkg/logx"
	"github.com/Muxi-X/muxi_auth_service_v2/pkg/oauth"
	"github.com/Muxi-X/muxi_auth_service_v2/router"
	"github.com/Muxi-X/muxi_auth_service_v2/router/middleware"

	"github.com/gin-gonic/gin"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	cfg = pflag.StringP("config", "c", "", "apiserver config file path.")
)

func main() {
	pflag.Parse()

	// init config
	if err := config.Init(*cfg); err != nil {
		panic(err)
	}

	// init db
	model.DB.Init()

	defer model.DB.Close()

	// init oauth-service
	oauth.OauthServer.Init()

	// Set gin mode.
	gin.SetMode(viper.GetString("runmode"))

	// Create the Gin engine.
	// 不用 gin.Default()：它的 access log 会把整条 query string 打出来，
	// 而 /auth/api/check_token 的 token 本身就是 7 天有效的登录凭据。
	// Recovery 由 router.Load 统一挂载。
	g := gin.New()
	g.Use(gin.LoggerWithFormatter(middleware.SensitiveLogFormatter))

	// Routes.
	router.Load(
		// Cores.
		g,
	)

	// Ping the server to make sure the router is working.
	go func() {
		if err := pingServer(); err != nil {
			logx.Fatal("The router has no response, or it might took too long to start up.", "error", err)
		}
		logx.Info("The router has been deployed successfully.")
	}()

	logx.Infof("Start to listening the incoming requests on http address: %s", viper.GetString("addr"))
	if err := http.ListenAndServe(viper.GetString("addr"), g); err != nil {
		logx.Error("HTTP server stopped", "error", err)
	}
}

// pingServer pings the http server to make sure the router is working.
func pingServer() error {
	for i := 0; i < viper.GetInt("max_ping_count"); i++ {
		// Ping the server by sending a GET request to `/health`.
		resp, err := http.Get(viper.GetString("url") + "/sd/health")
		if err == nil && resp.StatusCode == 200 {
			return nil
		}

		// Sleep for a second to continue the next ping.
		logx.Info("Waiting for the router, retry in 1 second.")
		time.Sleep(time.Second)
	}
	return errors.New("Cannot connect to the router.")
}

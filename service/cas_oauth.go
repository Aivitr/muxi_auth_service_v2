package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	pkgoauth "github.com/Muxi-X/muxi_auth_service_v2/pkg/oauth"
	"github.com/spf13/viper"
	cas "gopkg.in/cas.v2"
	oauth2 "gopkg.in/oauth2.v4"
	oauthserver "gopkg.in/oauth2.v4/server"
)

var (
	// ErrMissingCASTicket 表示 CAS callback 没有携带 ticket。
	ErrMissingCASTicket = errors.New("missing cas ticket")
	// ErrMissingOAuthClientID 表示 CAS callback 没有说明要给哪个 OAuth 客户端发码。
	ErrMissingOAuthClientID = errors.New("missing oauth client id")
	// ErrMissingCallbackURL 表示 callback 缺失最终要跳回的业务地址。
	ErrMissingCallbackURL = errors.New("missing callback url")
	// ErrInvalidCASTicket 表示 CAS 票据校验失败，常见于无效、过期或重复消费 ticket。
	ErrInvalidCASTicket = errors.New("invalid cas ticket")
)

// CASTicketValidator 抽象出 CAS 验票能力，便于测试替换真实网络交互。
type CASTicketValidator interface {
	ValidateTicket(serviceURL *url.URL, ticket string) (*cas.AuthenticationResponse, error)
}

// AuthorizeCodeGenerator 负责复用现有 OAuth 授权码发放逻辑。
type AuthorizeCodeGenerator interface {
	GenerateAuthorizeCode(ctx context.Context, request AuthorizeCodeRequest) (AuthorizeCodeResult, error)
}

// OAuthClientDomainResolver 负责根据业务回调域名反查已注册的 OAuth 客户端。
type OAuthClientDomainResolver interface {
	GetByDomain(domain string) (oauth2.ClientInfo, error)
}

// CASUserResolver 负责把 CAS 返回的外部身份解析为本地 users.id。
type CASUserResolver interface {
	ResolveCASUser(ctx context.Context, authenticationResponse *cas.AuthenticationResponse) (uint64, error)
}

// AuthorizeCodeRequest 描述生成 auth code 所需的最小输入。
type AuthorizeCodeRequest struct {
	ClientID       string
	UserID         string
	CallbackURL    string
	AccessTokenExp time.Duration
	// Scope 只让 oauth2_token.data 留下审计痕迹，没有任何代码读它，
	// 别指望这个字段承担成员校验职责。
	Scope string
}

// AuthorizeCodeResult 表示授权码生成结果。
type AuthorizeCodeResult struct {
	Code      string
	ExpiresIn time.Duration
}

// CASCallbackResult 表示 callback 成功处理后的对外结果。
type CASCallbackResult struct {
	Code        string
	RedirectURL string
}

// CASOAuthFlow 负责把“CAS 登录成功”翻译成“OAuth auth code 已签发”。
// 这层只负责编排，不关心 Gin handler 如何组织响应。
type CASOAuthFlow struct {
	callBackBaseURL string //这个其实是当前服务的域名,为什么需要这个呢?因为没办法直接从请求中获取到域名,但是cas验证是需要校验整个service的完整性的,所以需要域名
	ticketValidator CASTicketValidator
	codeGenerator   AuthorizeCodeGenerator
	casServerURL    *url.URL
	clientResolver  OAuthClientDomainResolver
	casUserResolver CASUserResolver
}

// NewCASOAuthFlow 创建一条可测试、可替换依赖的 CAS -> OAuth 回调流程。
func NewCASOAuthFlow(
	callBackBaseURL string,
	ticketValidator CASTicketValidator,
	codeGenerator AuthorizeCodeGenerator,
) *CASOAuthFlow {
	return NewCASOAuthFlowWithClientResolver(
		callBackBaseURL,
		nil,
		ticketValidator,
		codeGenerator,
		nil,
	)
}

func NewCASOAuthFlowWithClientResolver(
	callBackBaseURL string,
	casServerURL *url.URL,
	ticketValidator CASTicketValidator,
	codeGenerator AuthorizeCodeGenerator,
	clientResolver OAuthClientDomainResolver,
) *CASOAuthFlow {
	return NewCASOAuthFlowWithResolvers(
		callBackBaseURL,
		casServerURL,
		ticketValidator,
		codeGenerator,
		clientResolver,
		nil,
	)
}

func NewCASOAuthFlowWithResolvers(
	callBackBaseURL string,
	casServerURL *url.URL,
	ticketValidator CASTicketValidator,
	codeGenerator AuthorizeCodeGenerator,
	clientResolver OAuthClientDomainResolver,
	casUserResolver CASUserResolver,
) *CASOAuthFlow {
	var casServerURLCopy *url.URL
	if casServerURL != nil {
		copiedURL := *casServerURL
		casServerURLCopy = &copiedURL
	}

	return &CASOAuthFlow{
		callBackBaseURL: strings.TrimRight(callBackBaseURL, "/"),
		ticketValidator: ticketValidator,
		codeGenerator:   codeGenerator,
		casServerURL:    casServerURLCopy,
		clientResolver:  clientResolver,
		casUserResolver: casUserResolver,
	}
}

// HandleCallback 会完成以下事情：
// 1. 读取 callback 参数
// 2. 还原出 CAS service URL 并验票
// 3. 把 CAS 用户映射为本地用户
// 4. 复用现有 OAuth server 生成授权码
// 5. 组装最终回跳地址
func (f *CASOAuthFlow) HandleCallback(ctx context.Context, request *http.Request) (*CASCallbackResult, error) {
	// 获取签发的 ticket
	ticket := strings.TrimSpace(request.URL.Query().Get("ticket"))
	if ticket == "" {
		return nil, ErrMissingCASTicket
	}
	// 获取用户传输的重定向 url
	callbackURL := strings.TrimSpace(request.URL.Query().Get("callback_url"))
	if callbackURL == "" {
		return nil, ErrMissingCallbackURL
	}
	// 获取用户传输的 client_id
	clientID := strings.TrimSpace(request.URL.Query().Get("client_id"))
	if clientID == "" {
		retryURL, err := f.buildCASLoginRetryURL(request, callbackURL)
		if err != nil {
			return nil, err
		}

		return &CASCallbackResult{
			RedirectURL: retryURL,
		}, nil
	}
	if err := f.validateCallbackURLForClient(callbackURL, clientID); err != nil {
		return nil, err
	}
	// 构建完整的登陆url,为什么要携带这个呢?
	serviceURL, err := f.buildServiceURL(request)
	if err != nil {
		return nil, err
	}

	// 验证当前 ticket 的合法性
	authenticationResponse, err := f.ticketValidator.ValidateTicket(serviceURL, ticket)
	if err != nil || authenticationResponse == nil || authenticationResponse.User == "" {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCASTicket, err)
	}
	//从请求中去获取 code 的过期时间
	accessTokenExp, err := parseOptionalTokenExp(request.URL.Query().Get("token_exp"))
	if err != nil {
		return nil, err
	}

	userID, err := f.resolveCASLocalUserID(ctx, authenticationResponse)
	if err != nil {
		return nil, err
	}

	// 校验必须放在这个分支：上面缺 client_id 的 retry 分支既没有 userID，也不该被拦。
	scope := strings.TrimSpace(request.URL.Query().Get("scope"))
	if err := CheckMuxiMemberScope(userID, scope); err != nil {
		return nil, err
	}

	// 签发code
	codeResult, err := f.codeGenerator.GenerateAuthorizeCode(ctx, AuthorizeCodeRequest{
		ClientID:       clientID,
		UserID:         userID,
		CallbackURL:    callbackURL,
		AccessTokenExp: accessTokenExp,
		Scope:          scope,
	})
	if err != nil {
		return nil, err
	}

	redirectURL, err := appendCodeToCallbackURL(callbackURL, codeResult.Code)
	if err != nil {
		return nil, err
	}

	return &CASCallbackResult{
		Code:        codeResult.Code,
		RedirectURL: redirectURL,
	}, nil
}

func (f *CASOAuthFlow) resolveCASLocalUserID(ctx context.Context, authenticationResponse *cas.AuthenticationResponse) (string, error) {
	if f.casUserResolver == nil {
		return pkgoauth.BuildCASSubject(authenticationResponse.User), nil
	}

	userID, err := f.casUserResolver.ResolveCASUser(ctx, authenticationResponse)
	if err != nil {
		return "", err
	}
	return strconv.FormatUint(userID, 10), nil
}

// buildCASLoginRetryURL 兼容 CAS 短信二次认证后偶发丢失 client_id 的回调。
// 这里不能直接消费当前 ticket，因为 ticket 绑定的是缺 client_id 的 service URL；
// 正确做法是反查业务域名对应的 client_id，再让 CAS 用已有登录态签发一张新 ticket。
func (f *CASOAuthFlow) buildCASLoginRetryURL(request *http.Request, callbackURL string) (string, error) {
	clientID, err := f.resolveClientIDByCallbackURL(callbackURL)
	if err != nil {
		return "", err
	}

	serviceURL, err := f.buildRetryServiceURL(request, clientID, callbackURL)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrMissingOAuthClientID, err)
	}

	if f.casServerURL == nil || f.casServerURL.String() == "" {
		return "", fmt.Errorf("%w: cas server url is not configured", ErrMissingOAuthClientID)
	}

	loginURL := *f.casServerURL
	loginURL.Path = strings.TrimRight(loginURL.Path, "/") + "/login"
	query := loginURL.Query()
	query.Set("service", serviceURL.String())
	loginURL.RawQuery = query.Encode()

	return loginURL.String(), nil
}

func (f *CASOAuthFlow) resolveClientIDByCallbackURL(callbackURL string) (string, error) {
	clientInfo, err := f.resolveClientInfoByCallbackURL(callbackURL)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(clientInfo.GetID()), nil
}

func (f *CASOAuthFlow) validateCallbackURLForClient(callbackURL, clientID string) error {
	if f.clientResolver == nil {
		return nil
	}

	clientInfo, err := f.resolveClientInfoByCallbackURL(callbackURL)
	if err != nil {
		return err
	}
	if strings.TrimSpace(clientInfo.GetID()) != strings.TrimSpace(clientID) {
		return fmt.Errorf("%w: callback domain belongs to another oauth client", ErrMissingOAuthClientID)
	}
	if err := ValidateOAuthCallbackURLForDomain(clientInfo.GetDomain(), callbackURL); err != nil {
		return fmt.Errorf("%w: %v", ErrMissingCallbackURL, err)
	}
	return nil
}

func (f *CASOAuthFlow) resolveClientInfoByCallbackURL(callbackURL string) (oauth2.ClientInfo, error) {
	if f.clientResolver == nil {
		return nil, fmt.Errorf("%w: oauth client resolver is not configured", ErrMissingOAuthClientID)
	}

	parsedCallbackURL, err := url.Parse(callbackURL)
	if err != nil || parsedCallbackURL.Host == "" {
		return nil, fmt.Errorf("%w: invalid callback url", ErrMissingOAuthClientID)
	}

	var lastErr error
	for _, domain := range callbackDomainCandidates(parsedCallbackURL) {
		clientInfo, err := f.clientResolver.GetByDomain(domain)
		if err != nil {
			lastErr = err
			continue
		}
		if clientInfo == nil || strings.TrimSpace(clientInfo.GetID()) == "" {
			continue
		}
		if err := ValidateOAuthCallbackURLForDomain(clientInfo.GetDomain(), callbackURL); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrMissingCallbackURL, err)
		}

		return clientInfo, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("%w: cannot resolve oauth client for callback domain %s: %v", ErrMissingOAuthClientID, parsedCallbackURL.Host, lastErr)
	}

	return nil, fmt.Errorf("%w: callback domain %s is not registered", ErrMissingOAuthClientID, parsedCallbackURL.Host)
}

func callbackDomainCandidates(parsedURL *url.URL) []string {
	candidates := make([]string, 0, 3)
	addCandidate := func(domain string) {
		domain = strings.TrimSpace(domain)
		if domain == "" {
			return
		}
		for _, candidate := range candidates {
			if candidate == domain {
				return
			}
		}
		candidates = append(candidates, domain)
	}

	addCandidate(parsedURL.Host)
	addCandidate(strings.ToLower(parsedURL.Host))

	hostname := parsedURL.Hostname()
	port := parsedURL.Port()
	if port != "" && ((parsedURL.Scheme == "https" && port == "443") || (parsedURL.Scheme == "http" && port == "80")) {
		addCandidate(hostname)
		addCandidate(strings.ToLower(hostname))
	}

	return candidates
}

func (f *CASOAuthFlow) buildRetryServiceURL(request *http.Request, clientID, callbackURL string) (*url.URL, error) {
	parsedURI, err := url.ParseRequestURI(request.RequestURI)
	if err != nil {
		return nil, err
	}

	query := request.URL.Query()
	query.Del("ticket")
	query.Set("client_id", clientID)
	query.Set("callback_url", callbackURL)

	scheme, host := f.callbackSchemeAndHost(request)
	return &url.URL{
		Scheme:   scheme,
		Host:     host,
		Path:     parsedURI.Path,
		RawQuery: query.Encode(),
	}, nil
}

// buildServiceURL 会构造出与 CAS 当初签发 ticket 时一致的 service URL。
// 注意这里必须剔除 ticket 本身，否则 serviceValidate 时会因为 service 不一致而失败。
func (f *CASOAuthFlow) buildServiceURL(request *http.Request) (*url.URL, error) {
	// 1. 获取最原始、未经 Go 库重新拼装的完整 URL 字符串
	// request.RequestURI 包含了最原始的路径和参数
	rawURI := request.RequestURI

	// 2. 从原始 query 中移除 ticket，同时保留其他参数的原始顺序和编码。
	// CAS 回调会把 ticket 放到 query 的任意位置，不能简单截断字符串。
	parsedClean, err := url.ParseRequestURI(rawURI)
	if err != nil {
		return nil, err
	}
	parsedClean.RawQuery = removeQueryParam(parsedClean.RawQuery, "ticket")

	// 3. 构建返回的 URL 对象
	// 依然保留你之前的 Scheme 和 Host 处理逻辑，确保验证请求发往正确的地址
	scheme, host := f.callbackSchemeAndHost(request)

	return &url.URL{
		Scheme:   scheme,
		Host:     host,
		Path:     parsedClean.Path,
		RawQuery: parsedClean.RawQuery, // 这里拿到的就是完全没被重排过的原始 Query 字符串
	}, nil
}

func (f *CASOAuthFlow) callbackSchemeAndHost(request *http.Request) (string, string) {
	var scheme string
	if request.TLS != nil || request.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	} else {
		scheme = "http"
	}

	host := request.Host
	if f.callBackBaseURL != "" {
		u, _ := url.Parse(f.callBackBaseURL)
		scheme = u.Scheme
		host = u.Host
	}

	return scheme, host
}

func removeQueryParam(rawQuery, key string) string {
	if rawQuery == "" {
		return ""
	}

	parts := strings.Split(rawQuery, "&")
	filtered := parts[:0]
	for _, part := range parts {
		if part == "" {
			continue
		}

		paramKey := part
		if idx := strings.Index(part, "="); idx != -1 {
			paramKey = part[:idx]
		}
		if paramKey == key {
			continue
		}
		filtered = append(filtered, part)
	}
	return strings.Join(filtered, "&")
}

// parseOptionalTokenExp 用来兼容现有 OAuth 接口的 token_exp 参数语义。
// 如果没有传值，就返回 0，表示沿用服务当前默认的 access token 过期时间。
func parseOptionalTokenExp(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}

	expireSeconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid token_exp: %w", err)
	}

	if expireSeconds <= 0 {
		return 0, nil
	}

	return time.Duration(expireSeconds) * time.Second, nil
}

// appendCodeToCallbackURL 会把 code 安全地拼接回 callback_url。
// 这里显式使用 URL API，是为了兼容 callback_url 本身已有查询参数的情况。
func appendCodeToCallbackURL(callbackURL, code string) (string, error) {
	parsedURL, err := url.Parse(callbackURL)
	if err != nil {
		return "", err
	}

	query := parsedURL.Query()
	query.Set("code", code)
	parsedURL.RawQuery = query.Encode()

	return parsedURL.String(), nil
}

// defaultCASTicketValidator 是生产环境使用的 CAS 验票实现。
type defaultCASTicketValidator struct {
	validator *cas.ServiceTicketValidator
}

func (d *defaultCASTicketValidator) ValidateTicket(serviceURL *url.URL, ticket string) (*cas.AuthenticationResponse, error) {
	return d.validator.ValidateTicket(serviceURL, ticket)
}

// defaultAuthorizeCodeGenerator 复用现有 OAuth server 的发码逻辑，
// 避免新 CAS 流生成出来的 code 脱离原有 token 存储与过期策略。
type defaultAuthorizeCodeGenerator struct{}

func (g *defaultAuthorizeCodeGenerator) GenerateAuthorizeCode(ctx context.Context, request AuthorizeCodeRequest) (AuthorizeCodeResult, error) {
	authorizeRequest := &oauthserver.AuthorizeRequest{
		ResponseType:   oauth2.Code,
		ClientID:       request.ClientID,
		RedirectURI:    request.CallbackURL,
		UserID:         request.UserID,
		AccessTokenExp: request.AccessTokenExp,
		Scope:          request.Scope,
	}

	tokenInfo, err := pkgoauth.OauthServer.Server.GetAuthorizeToken(ctx, authorizeRequest)
	if err != nil {
		return AuthorizeCodeResult{}, err
	}

	return AuthorizeCodeResult{
		Code:      tokenInfo.GetCode(),
		ExpiresIn: tokenInfo.GetCodeExpiresIn(),
	}, nil
}

// NewDefaultCASOAuthFlow 会基于配置创建生产环境使用的默认流程实现。
func NewDefaultCASOAuthFlow() (*CASOAuthFlow, error) {
	casServerURL, err := url.Parse(strings.TrimSpace(viper.GetString("cas.server_url")))
	if err != nil {
		return nil, err
	}

	if casServerURL.String() == "" {
		return nil, errors.New("cas.server_url is required")
	}

	validator := cas.NewServiceTicketValidator(http.DefaultClient, casServerURL)
	var clientResolver OAuthClientDomainResolver
	if pkgoauth.OauthServer != nil {
		clientResolver = pkgoauth.OauthServer.ClientStore
	}

	return NewCASOAuthFlowWithClientResolver(
		viper.GetString("cas.callback_base_url"),
		casServerURL,
		&defaultCASTicketValidator{validator: validator},
		&defaultAuthorizeCodeGenerator{},
		clientResolver,
	).withCASUserResolver(&defaultCASUserResolver{}), nil
}

func (f *CASOAuthFlow) withCASUserResolver(casUserResolver CASUserResolver) *CASOAuthFlow {
	f.casUserResolver = casUserResolver
	return f
}

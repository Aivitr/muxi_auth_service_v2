package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/Muxi-X/muxi_auth_service_v2/model"
	"github.com/Muxi-X/muxi_auth_service_v2/pkg/errno"

	cas "gopkg.in/cas.v2"
	oauth2 "gopkg.in/oauth2.v4"
	"gopkg.in/oauth2.v4/models"
)

// newScopeTestFlow 里的 generated 用来数发码器被调用了几次。
func newScopeTestFlow(t *testing.T, localUserID uint64, generated *int) *CASOAuthFlow {
	t.Helper()

	return NewCASOAuthFlowWithResolvers(
		"",
		nil,
		fakeCASTicketValidator{
			validateFunc: func(serviceURL *url.URL, ticket string) (*cas.AuthenticationResponse, error) {
				return &cas.AuthenticationResponse{User: "casuser"}, nil
			},
		},
		fakeAuthorizeCodeGenerator{
			generateFunc: func(ctx context.Context, request AuthorizeCodeRequest) (AuthorizeCodeResult, error) {
				*generated++
				return AuthorizeCodeResult{Code: "auth-code", ExpiresIn: 0}, nil
			},
		},
		nil,
		fakeCASUserResolver{
			resolveFunc: func(ctx context.Context, authenticationResponse *cas.AuthenticationResponse) (uint64, error) {
				return localUserID, nil
			},
		},
	)
}

func createScopeTestUsers(t *testing.T) (*model.UserModel, *model.UserModel) {
	t.Helper()

	db := setupCASIdentityTestDB(t)

	member := &model.UserModel{Email: "member@example.com", Username: "member", RoleID: 3}
	outsider := &model.UserModel{Email: "outsider@example.com", Username: "outsider", RoleID: 3}
	for _, user := range []*model.UserModel{member, outsider} {
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("create user failed: %v", err)
		}
	}
	if err := (&model.MemberProfile{UserID: member.Id, Group: "Backend"}).Create(); err != nil {
		t.Fatalf("create profile failed: %v", err)
	}

	return member, outsider
}

func TestCASOAuthFlowHandleCallbackRejectsNonMemberRequestingMuxiScope(t *testing.T) {
	_, outsider := createScopeTestUsers(t)

	generated := 0
	flow := newScopeTestFlow(t, outsider.Id, &generated)

	req := httptest.NewRequest(http.MethodGet,
		"http://oauth.example.com/auth/api/oauth/cas/callback?ticket=ST-1&client_id=client-a&callback_url=https%3A%2F%2Fclient.example.com%2Fcb&scope=muxi:member", nil)

	result, err := flow.HandleCallback(context.Background(), req)
	if !errors.Is(err, errno.ErrNotMuxiMember) {
		t.Fatalf("expected ErrNotMuxiMember, got %v", err)
	}
	if result != nil {
		t.Fatalf("expected no result, got %+v", result)
	}
	// 发码器一次都不能被调用 —— 这是「没发码」的唯一证据。
	if generated != 0 {
		t.Fatalf("expected the code generator to never run, ran %d times", generated)
	}
}

func TestCASOAuthFlowHandleCallbackPassesScopeThroughForMember(t *testing.T) {
	member, _ := createScopeTestUsers(t)

	var captured AuthorizeCodeRequest
	generated := 0
	flow := NewCASOAuthFlowWithResolvers(
		"",
		nil,
		fakeCASTicketValidator{
			validateFunc: func(serviceURL *url.URL, ticket string) (*cas.AuthenticationResponse, error) {
				// scope 必须能被 buildServiceURL 完整带到 CAS 那一侧，否则前端塞的会中途丢掉。
				if serviceURL.Query().Get("scope") != "muxi:member" {
					t.Fatalf("expected scope to survive into the service url, got %q", serviceURL.Query().Get("scope"))
				}
				return &cas.AuthenticationResponse{User: "casuser"}, nil
			},
		},
		fakeAuthorizeCodeGenerator{
			generateFunc: func(ctx context.Context, request AuthorizeCodeRequest) (AuthorizeCodeResult, error) {
				generated++
				captured = request
				return AuthorizeCodeResult{Code: "auth-code", ExpiresIn: 0}, nil
			},
		},
		nil,
		fakeCASUserResolver{
			resolveFunc: func(ctx context.Context, authenticationResponse *cas.AuthenticationResponse) (uint64, error) {
				return member.Id, nil
			},
		},
	)

	req := httptest.NewRequest(http.MethodGet,
		"http://oauth.example.com/auth/api/oauth/cas/callback?ticket=ST-1&client_id=client-a&callback_url=https%3A%2F%2Fclient.example.com%2Fcb&scope=muxi:member", nil)

	result, err := flow.HandleCallback(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleCallback() returned error: %v", err)
	}
	if generated != 1 {
		t.Fatalf("expected the code generator to run once, ran %d times", generated)
	}
	if captured.Scope != "muxi:member" {
		t.Fatalf("expected scope to be forwarded to the generator, got %q", captured.Scope)
	}
	if captured.UserID != strconv.FormatUint(member.Id, 10) {
		t.Fatalf("expected user id %d, got %s", member.Id, captured.UserID)
	}
	if result.Code != "auth-code" {
		t.Fatalf("expected code auth-code, got %s", result.Code)
	}
}

func TestCASOAuthFlowRetryBranchDoesNotCheckMembership(t *testing.T) {
	// 缺 client_id 的 retry 分支既没有 userID，也不该被成员校验拦下。
	// DB 置空是为了证明这条路径根本没碰数据库。
	oldDB := model.DB
	model.DB = nil
	t.Cleanup(func() {
		model.DB = oldDB
	})

	casServerURL, err := url.Parse("https://account.example.edu/cas")
	if err != nil {
		t.Fatalf("parse cas server url failed: %v", err)
	}

	generated := 0
	flow := NewCASOAuthFlowWithClientResolver(
		"https://pass.example.com",
		casServerURL,
		fakeCASTicketValidator{
			validateFunc: func(serviceURL *url.URL, ticket string) (*cas.AuthenticationResponse, error) {
				t.Fatalf("ticket validator should not be called when client_id is missing")
				return nil, nil
			},
		},
		fakeAuthorizeCodeGenerator{
			generateFunc: func(ctx context.Context, request AuthorizeCodeRequest) (AuthorizeCodeResult, error) {
				generated++
				return AuthorizeCodeResult{}, nil
			},
		},
		fakeOAuthClientDomainResolver{
			getFunc: func(domain string) (oauth2.ClientInfo, error) {
				return &models.Client{
					ID:     "client-from-domain",
					Domain: "https://forum-dev.muxistudio.xyz",
				}, nil
			},
		},
	)

	req := httptest.NewRequest(http.MethodGet,
		"https://pass.example.com/auth/api/oauth/cas/callback?ticket=ST-2&callback_url=https://forum-dev.muxistudio.xyz/login/student-oauth&scope=muxi:member", nil)

	result, err := flow.HandleCallback(context.Background(), req)
	if err != nil {
		t.Fatalf("expected the retry branch to succeed, got %v", err)
	}
	if result == nil || result.RedirectURL == "" {
		t.Fatalf("expected a CAS login retry url, got %+v", result)
	}
	if generated != 0 {
		t.Fatalf("expected no code to be generated, ran %d times", generated)
	}

	// scope 必须原样带进重试的 service URL，否则第二次进来就丢了。
	loginURL, err := url.Parse(result.RedirectURL)
	if err != nil {
		t.Fatalf("parse retry redirect url failed: %v", err)
	}
	serviceURL, err := url.Parse(loginURL.Query().Get("service"))
	if err != nil {
		t.Fatalf("parse retry service url failed: %v", err)
	}
	if serviceURL.Query().Get("scope") != "muxi:member" {
		t.Fatalf("expected scope to survive the retry, got %q", serviceURL.Query().Get("scope"))
	}
}

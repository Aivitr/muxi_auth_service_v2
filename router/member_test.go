package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMemberAdminRoutesRequireAdminToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := Load(gin.New())

	requests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/auth/api/admin/members", ""},
		{http.MethodPost, "/auth/api/admin/members", `{"user_id":1,"group":"Backend"}`},
		{http.MethodPut, "/auth/api/admin/members/1", `{"group":"Design"}`},
		{http.MethodDelete, "/auth/api/admin/members/1", ""},
		{http.MethodGet, "/auth/api/admin/users/search?q=alice", ""},
	}

	for _, rq := range requests {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(rq.method, rq.path, strings.NewReader(rq.body))
		req.Header.Set("Content-Type", "application/json")

		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: expected status 401, got %d: %s", rq.method, rq.path, w.Code, w.Body.String())
		}
	}
}

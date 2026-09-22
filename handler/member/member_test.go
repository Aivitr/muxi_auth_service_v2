package member

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Muxi-X/muxi_auth_service_v2/model"
	"github.com/Muxi-X/muxi_auth_service_v2/pkg/errno"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

type apiResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func setupMemberHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open test database failed: %v", err)
	}
	db.LogMode(false)
	if err := db.AutoMigrate(&model.UserModel{}, &model.MemberProfile{}).Error; err != nil {
		db.Close()
		t.Fatalf("migrate test database failed: %v", err)
	}

	oldDB := model.DB
	model.DB = &model.Database{Self: db}
	t.Cleanup(func() {
		model.DB = oldDB
		db.Close()
	})

	return db
}

func createHandlerTestUser(t *testing.T, db *gorm.DB, username string) *model.UserModel {
	t.Helper()

	user := &model.UserModel{
		Email:    username + "@example.com",
		Username: username,
		RoleID:   3,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user %q failed: %v", username, err)
	}

	return user
}

func callHandler(t *testing.T, h gin.HandlerFunc, method, target string, body interface{}, params gin.Params) (int, apiResponse) {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body failed: %v", err)
		}
		reader = bytes.NewReader(payload)
	} else {
		reader = bytes.NewReader(nil)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, target, reader)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = params

	h(c)

	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v (body=%s)", err, w.Body.String())
	}

	return w.Code, resp
}

func TestCreateRejectsInvalidGroup(t *testing.T) {
	db := setupMemberHandlerTestDB(t)
	user := createHandlerTestUser(t, db, "alice")

	status, resp := callHandler(t, Create, http.MethodPost, "/auth/api/admin/members", MemberRequest{
		UserID: user.Id,
		Group:  "Android",
	}, nil)

	if status != http.StatusBadRequest || resp.Code != errno.ErrInvalidMemberGroup.Code {
		t.Fatalf("expected 400/%d, got %d/%d (%s)", errno.ErrInvalidMemberGroup.Code, status, resp.Code, resp.Message)
	}
}

func TestCreateRejectsUnknownUser(t *testing.T) {
	setupMemberHandlerTestDB(t)

	status, resp := callHandler(t, Create, http.MethodPost, "/auth/api/admin/members", MemberRequest{
		UserID: 9999,
		Group:  "Backend",
	}, nil)

	if status != http.StatusNotFound || resp.Code != errno.ErrUserNotFound.Code {
		t.Fatalf("expected 404/%d, got %d/%d (%s)", errno.ErrUserNotFound.Code, status, resp.Code, resp.Message)
	}
}

func TestCreateRejectsDuplicateMember(t *testing.T) {
	db := setupMemberHandlerTestDB(t)
	user := createHandlerTestUser(t, db, "bob")

	status, _ := callHandler(t, Create, http.MethodPost, "/auth/api/admin/members", MemberRequest{
		UserID: user.Id,
		Group:  "Backend",
	}, nil)
	if status != http.StatusOK {
		t.Fatalf("expected the first authorization to succeed, got %d", status)
	}

	status, resp := callHandler(t, Create, http.MethodPost, "/auth/api/admin/members", MemberRequest{
		UserID: user.Id,
		Group:  "Design",
	}, nil)

	if status != http.StatusBadRequest || resp.Code != errno.ErrMemberProfileExisted.Code {
		t.Fatalf("expected 400/%d, got %d/%d (%s)", errno.ErrMemberProfileExisted.Code, status, resp.Code, resp.Message)
	}
}

func TestCreateRejectsTakenStudentID(t *testing.T) {
	db := setupMemberHandlerTestDB(t)
	first := createHandlerTestUser(t, db, "carol")
	second := createHandlerTestUser(t, db, "dave")

	status, _ := callHandler(t, Create, http.MethodPost, "/auth/api/admin/members", MemberRequest{
		UserID:    first.Id,
		StudentID: "2023001",
		Group:     "Backend",
	}, nil)
	if status != http.StatusOK {
		t.Fatalf("expected the first authorization to succeed, got %d", status)
	}

	status, resp := callHandler(t, Create, http.MethodPost, "/auth/api/admin/members", MemberRequest{
		UserID:    second.Id,
		StudentID: "2023001",
		Group:     "Backend",
	}, nil)

	if status != http.StatusBadRequest || resp.Code != errno.ErrStudentIDExisted.Code {
		t.Fatalf("expected 400/%d, got %d/%d (%s)", errno.ErrStudentIDExisted.Code, status, resp.Code, resp.Message)
	}
}

func TestCreateAllowsEmptyStudentIDForMultipleMembers(t *testing.T) {
	db := setupMemberHandlerTestDB(t)

	// 迁移后大量成员学号为空，唯一性约束必须对空串豁免，否则第二个成员就建不了。
	for _, username := range []string{"erin", "frank"} {
		user := createHandlerTestUser(t, db, username)
		status, resp := callHandler(t, Create, http.MethodPost, "/auth/api/admin/members", MemberRequest{
			UserID: user.Id,
			Group:  "Product",
		}, nil)
		if status != http.StatusOK {
			t.Fatalf("expected empty student id to be accepted for %s, got %d/%d (%s)",
				username, status, resp.Code, resp.Message)
		}
	}
}

func TestUpdateRejectsInvalidGroupAndUnknownMember(t *testing.T) {
	db := setupMemberHandlerTestDB(t)
	user := createHandlerTestUser(t, db, "grace")

	params := gin.Params{{Key: "user_id", Value: "1"}}

	status, resp := callHandler(t, Update, http.MethodPut, "/auth/api/admin/members/1", MemberRequest{
		Group: "iOS",
	}, params)
	if status != http.StatusBadRequest || resp.Code != errno.ErrInvalidMemberGroup.Code {
		t.Fatalf("expected 400/%d, got %d/%d (%s)", errno.ErrInvalidMemberGroup.Code, status, resp.Code, resp.Message)
	}

	params = gin.Params{{Key: "user_id", Value: "9999"}}
	status, resp = callHandler(t, Update, http.MethodPut, "/auth/api/admin/members/9999", MemberRequest{
		Group: "Backend",
	}, params)
	if status != http.StatusNotFound || resp.Code != errno.ErrMemberProfileNotFound.Code {
		t.Fatalf("expected 404/%d, got %d/%d (%s)", errno.ErrMemberProfileNotFound.Code, status, resp.Code, resp.Message)
	}

	// 参数不是数字时不该把 0 当成 user_id 去查。
	params = gin.Params{{Key: "user_id", Value: "abc"}}
	status, _ = callHandler(t, Update, http.MethodPut, "/auth/api/admin/members/abc", MemberRequest{
		Group: "Backend",
	}, params)
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 for a non-numeric user_id, got %d", status)
	}

	if _, err := model.GetMemberProfileByUserID(user.Id); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateKeepsOwnStudentID(t *testing.T) {
	db := setupMemberHandlerTestDB(t)
	user := createHandlerTestUser(t, db, "heidi")

	status, _ := callHandler(t, Create, http.MethodPost, "/auth/api/admin/members", MemberRequest{
		UserID:    user.Id,
		StudentID: "2023002",
		Group:     "Backend",
	}, nil)
	if status != http.StatusOK {
		t.Fatalf("expected the authorization to succeed, got %d", status)
	}

	// 更新自己时不该被自己的学号挡住（excludeUserID 必须传自己）。
	params := gin.Params{{Key: "user_id", Value: itoa(user.Id)}}
	status, resp := callHandler(t, Update, http.MethodPut, "/auth/api/admin/members/"+itoa(user.Id), MemberRequest{
		StudentID: "2023002",
		RealName:  "海蒂",
	}, params)
	if status != http.StatusOK {
		t.Fatalf("expected the update to succeed, got %d/%d (%s)", status, resp.Code, resp.Message)
	}

	profile, err := model.GetMemberProfileByUserID(user.Id)
	if err != nil || profile == nil {
		t.Fatalf("expected the profile to survive the update, err=%v", err)
	}
	if profile.RealName != "海蒂" || profile.StudentID != "2023002" || profile.Group != "Backend" {
		t.Fatalf("expected real_name to be updated and the rest preserved, got %+v", profile)
	}
}

func TestDeleteRemovesMemberProfile(t *testing.T) {
	db := setupMemberHandlerTestDB(t)
	user := createHandlerTestUser(t, db, "ivan")

	status, _ := callHandler(t, Create, http.MethodPost, "/auth/api/admin/members", MemberRequest{
		UserID: user.Id,
		Group:  "Design",
	}, nil)
	if status != http.StatusOK {
		t.Fatalf("expected the authorization to succeed, got %d", status)
	}

	params := gin.Params{{Key: "user_id", Value: itoa(user.Id)}}
	status, resp := callHandler(t, Delete, http.MethodDelete, "/auth/api/admin/members/"+itoa(user.Id), nil, params)
	if status != http.StatusOK {
		t.Fatalf("expected the deletion to succeed, got %d/%d (%s)", status, resp.Code, resp.Message)
	}

	profile, err := model.GetMemberProfileByUserID(user.Id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile != nil {
		t.Fatalf("expected the profile to be gone, got %+v", profile)
	}

	// 再删一次应该 404，而不是静默成功。
	status, resp = callHandler(t, Delete, http.MethodDelete, "/auth/api/admin/members/"+itoa(user.Id), nil, params)
	if status != http.StatusNotFound || resp.Code != errno.ErrMemberProfileNotFound.Code {
		t.Fatalf("expected 404/%d on the second delete, got %d/%d", errno.ErrMemberProfileNotFound.Code, status, resp.Code)
	}
}

func TestListClampsPageSize(t *testing.T) {
	db := setupMemberHandlerTestDB(t)
	user := createHandlerTestUser(t, db, "judy")
	if err := (&model.MemberProfile{UserID: user.Id, RealName: "朱迪", Group: "Backend"}).Create(); err != nil {
		t.Fatalf("create profile failed: %v", err)
	}

	status, resp := callHandler(t, List, http.MethodGet, "/auth/api/admin/members?page_size=500", nil, nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	var payload MemberListResponse
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		t.Fatalf("decode list failed: %v", err)
	}
	if payload.PageSize != maxPageSize || payload.Page != 1 || payload.Total != 1 || len(payload.List) != 1 {
		t.Fatalf("expected page_size to be clamped to %d, got %+v", maxPageSize, payload)
	}
}

func TestSearchUsersRequiresKeyword(t *testing.T) {
	db := setupMemberHandlerTestDB(t)
	createHandlerTestUser(t, db, "karl")

	status, _ := callHandler(t, SearchUsers, http.MethodGet, "/auth/api/admin/users/search", nil, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 without q, got %d", status)
	}

	status, resp := callHandler(t, SearchUsers, http.MethodGet, "/auth/api/admin/users/search?q=karl", nil, nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	var payload UserSearchResponse
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		t.Fatalf("decode search result failed: %v", err)
	}
	if len(payload.List) != 1 || payload.List[0].Username != "karl" {
		t.Fatalf("expected to find karl, got %+v", payload.List)
	}
}

func itoa(value uint64) string {
	return strconv.FormatUint(value, 10)
}

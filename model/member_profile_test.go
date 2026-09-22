package model

import (
	"testing"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func setupMemberProfileTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open test database failed: %v", err)
	}
	db.LogMode(false)
	if err := db.AutoMigrate(&UserModel{}, &MemberProfile{}, &Role{}).Error; err != nil {
		db.Close()
		t.Fatalf("migrate test database failed: %v", err)
	}

	oldDB := DB
	DB = &Database{Self: db}
	t.Cleanup(func() {
		DB = oldDB
		db.Close()
	})

	return db
}

func createTestUser(t *testing.T, db *gorm.DB, username string) *UserModel {
	t.Helper()

	user := &UserModel{
		Email:    username + "@example.com",
		Username: username,
		RoleID:   3,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user %q failed: %v", username, err)
	}

	return user
}

func TestCreateMemberProfileFillsTimestamps(t *testing.T) {
	setupMemberProfileTestDB(t)
	user := createTestUser(t, DB.Self, "alice")

	profile := &MemberProfile{
		UserID:   user.Id,
		RealName: "爱丽丝",
		Group:    "Backend",
		JoinYear: 2023,
	}
	if err := profile.Create(); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if profile.CreatedAt.IsZero() || profile.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps to be set, got created=%v updated=%v", profile.CreatedAt, profile.UpdatedAt)
	}
}

func TestGetMemberProfileByUserIDReturnsNilForNonMember(t *testing.T) {
	setupMemberProfileTestDB(t)
	user := createTestUser(t, DB.Self, "bob")

	profile, err := GetMemberProfileByUserID(user.Id)
	if err != nil {
		t.Fatalf("expected non-member to be a normal state, got error: %v", err)
	}
	if profile != nil {
		t.Fatalf("expected nil profile for non-member, got %+v", profile)
	}

	isMember, err := IsMuxiMember(user.Id)
	if err != nil {
		t.Fatalf("IsMuxiMember returned error: %v", err)
	}
	if isMember {
		t.Fatal("expected non-member to report false")
	}
}

func TestIsStudentIDTaken(t *testing.T) {
	setupMemberProfileTestDB(t)
	member := createTestUser(t, DB.Self, "carol")
	other := createTestUser(t, DB.Self, "dave")

	if err := (&MemberProfile{UserID: member.Id, StudentID: "2023001", Group: "Backend"}).Create(); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	// 空学号不参与唯一性约束：迁移后会有大量空学号，约束它们会让补录无从下手。
	taken, err := IsStudentIDTaken("", 0)
	if err != nil {
		t.Fatalf("IsStudentIDTaken returned error: %v", err)
	}
	if taken {
		t.Fatal("expected empty student id to be exempt from uniqueness")
	}

	taken, err = IsStudentIDTaken("2023001", other.Id)
	if err != nil {
		t.Fatalf("IsStudentIDTaken returned error: %v", err)
	}
	if !taken {
		t.Fatal("expected duplicated student id to be reported as taken")
	}

	// 更新自己时不该被自己的学号挡住。
	taken, err = IsStudentIDTaken("2023001", member.Id)
	if err != nil {
		t.Fatalf("IsStudentIDTaken returned error: %v", err)
	}
	if taken {
		t.Fatal("expected excludeUserID to exclude the member itself")
	}
}

func TestListMemberProfilesFiltersByGroupAndScansColumns(t *testing.T) {
	setupMemberProfileTestDB(t)
	backendUser := createTestUser(t, DB.Self, "erin")
	frontendUser := createTestUser(t, DB.Self, "frank")

	profiles := []*MemberProfile{
		{UserID: backendUser.Id, RealName: "艾琳", StudentID: "2023002", Group: "Backend", JoinYear: 2022},
		{UserID: frontendUser.Id, RealName: "弗兰克", StudentID: "2023003", Group: "Frontend"},
	}
	for _, profile := range profiles {
		if err := profile.Create(); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
	}

	// group 是 SQL 保留字，裸写会在 sqlite 上直接语法报错，
	// 这条用例顺带守住「忘了加反引号」的回归。
	items, total, err := ListMemberProfiles(0, 20, "Backend", "")
	if err != nil {
		t.Fatalf("ListMemberProfiles returned error: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected exactly 1 backend member, got total=%d len=%d", total, len(items))
	}

	// Scan 时列名与字段名对不上会静默填零值，这里逐字段确认真的取到了值。
	got := items[0]
	if got.UserID != backendUser.Id || got.Username != "erin" || got.Email != "erin@example.com" {
		t.Fatalf("expected joined user columns to be populated, got %+v", got)
	}
	if got.RealName != "艾琳" || got.StudentID != "2023002" || got.Group != "Backend" {
		t.Fatalf("expected profile columns to be populated, got %+v", got)
	}
	if got.JoinYear != 2022 {
		t.Fatalf("expected join_year 2022, got %d", got.JoinYear)
	}

	items, total, err = ListMemberProfiles(0, 20, "", "2023003")
	if err != nil {
		t.Fatalf("ListMemberProfiles returned error: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].Username != "frank" {
		t.Fatalf("expected keyword search to match by student id, got total=%d items=%+v", total, items)
	}
}

func TestListMemberProfilesIncludesProfileWithNullJoinYear(t *testing.T) {
	setupMemberProfileTestDB(t)
	user := createTestUser(t, DB.Self, "grace")

	profile := &MemberProfile{UserID: user.Id, RealName: "格蕾丝", Group: "Design"}
	if err := profile.Create(); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	// 模拟迁移脚本里 timejoin 解析不出年份的那批数据。
	if err := DB.Self.Exec("UPDATE member_profiles SET join_year = NULL WHERE user_id = ?", user.Id).Error; err != nil {
		t.Fatalf("reset join_year failed: %v", err)
	}

	items, total, err := ListMemberProfiles(0, 20, "", "")
	if err != nil {
		t.Fatalf("ListMemberProfiles returned error: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected 1 member, got total=%d len=%d", total, len(items))
	}
	if items[0].JoinYear != 0 {
		t.Fatalf("expected NULL join_year to scan as 0, got %d", items[0].JoinYear)
	}
}

func TestSearchUsers(t *testing.T) {
	setupMemberProfileTestDB(t)
	member := createTestUser(t, DB.Self, "heidi")
	outsider := createTestUser(t, DB.Self, "ivan")

	if err := (&MemberProfile{UserID: member.Id, RealName: "海蒂", StudentID: "2023004", Group: "Product"}).Create(); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	items, err := SearchUsers("", 20)
	if err != nil {
		t.Fatalf("SearchUsers returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty keyword to return nothing, got %+v", items)
	}

	items, err = SearchUsers("2023004", 20)
	if err != nil {
		t.Fatalf("SearchUsers returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(items))
	}
	if items[0].UserID != member.Id || !items[0].IsMuxiMember || items[0].RealName != "海蒂" {
		t.Fatalf("expected the member to be flagged, got %+v", items[0])
	}

	items, err = SearchUsers("ivan", 20)
	if err != nil {
		t.Fatalf("SearchUsers returned error: %v", err)
	}
	if len(items) != 1 || items[0].UserID != outsider.Id {
		t.Fatalf("expected to find the non-member by username, got %+v", items)
	}
	if items[0].IsMuxiMember {
		t.Fatal("expected the non-member to be flagged false")
	}
}

func TestRevokeUserOAuthTokensSkipsMissingTable(t *testing.T) {
	setupMemberProfileTestDB(t)

	// oauth2_token 由依赖在服务启动时建表，测试库里没有。
	// 这个守卫保证删除成员时不会因为找不到表而整个请求失败。
	if err := RevokeUserOAuthTokens(1); err != nil {
		t.Fatalf("expected missing oauth2_token table to be skipped, got %v", err)
	}
}

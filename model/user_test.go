package model

import (
	"encoding/json"
	"testing"

	"github.com/Muxi-X/muxi_auth_service_v2/pkg/constvar"
)

func TestHasAdminAccess(t *testing.T) {
	db := setupMemberProfileTestDB(t)

	if HasAdminAccess(nil) {
		t.Fatal("expected nil user to be rejected")
	}
	if !HasAdminAccess(&UserModel{RoleID: RoleIDAdmin}) {
		t.Fatal("expected administrator role to have admin access")
	}

	// 非 admin 走权限位。db.sql 里 Moderator 的权限值 14 不含这一位，另造一个角色验分支。
	const customRoleID = 7
	if err := db.Create(&Role{
		BaseModel:   BaseModel{Id: customRoleID},
		Name:        "MemberAdmin",
		Permissions: constvar.PermissionOAuthClientManage,
	}).Error; err != nil {
		t.Fatalf("create role failed: %v", err)
	}
	if !HasAdminAccess(&UserModel{RoleID: customRoleID}) {
		t.Fatal("expected the permission bit to grant admin access")
	}

	// 锁住既有行为：roles 表里没有 role_id = 3 这一行（db.sql 只种了三个角色），
	// 查不到权限位必须拒绝；Moderator 的权限值 14 不含该位，同样进不了管理端。
	for _, roleID := range []uint64{RoleIDUser, RoleIDModerator} {
		if HasAdminAccess(&UserModel{RoleID: roleID}) {
			t.Fatalf("expected role_id %d to be rejected", roleID)
		}
	}
}

func TestBuildUserRolesNeverReturnsNil(t *testing.T) {
	tests := []struct {
		name     string
		roleID   uint64
		isMember bool
		want     []string
	}{
		{"plain student", RoleIDUser, false, []string{"user"}},
		{"plain member", RoleIDUser, true, []string{"user", "muxi_member"}},
		{"moderator member", RoleIDModerator, true, []string{"user", "muxi_member", "moderator"}},
		{"administrator", RoleIDAdmin, false, []string{"user", "admin"}},
		{"unknown role id", 99, false, []string{"user"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildUserRoles(&UserModel{RoleID: tt.roleID}, tt.isMember)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("expected %v, got %v", tt.want, got)
				}
			}

			// json.Marshal(nil slice) 产出 null，前端 roles.length 会炸。
			encoded, err := json.Marshal(&UserInfo{Roles: got})
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}
			if !json.Valid(encoded) || string(encoded) == `{"roles":null}` {
				t.Fatalf("expected roles to serialize as an array, got %s", encoded)
			}
		})
	}
}

func TestGetUserInfoByIDForNonMember(t *testing.T) {
	setupMemberProfileTestDB(t)
	user := createTestUser(t, DB.Self, "alice")
	user.Group = "前端"
	user.PersonalBlog = "https://blog.example.com"
	if err := user.Update(); err != nil {
		t.Fatalf("update user failed: %v", err)
	}

	info, err := GetUserInfoByID(user.Id)
	if err != nil {
		t.Fatalf("GetUserInfoByID returned error: %v", err)
	}

	if info.IsMuxiMember {
		t.Fatal("expected a user without a profile to be a non-member")
	}
	if info.MemberProfile != nil {
		t.Fatalf("expected null member_profile for non-member, got %+v", info.MemberProfile)
	}
	if len(info.Roles) != 1 || info.Roles[0] != RoleNameUser {
		t.Fatalf("expected roles [user], got %v", info.Roles)
	}
	// member_profiles 缺失不该影响 users 上的存量字段。
	if info.Group != "前端" || info.PersonalBlog != "https://blog.example.com" {
		t.Fatalf("expected legacy users fields to be preserved, got %+v", info)
	}
}

func TestGetUserInfoByIDForMemberBackfillsLegacyFields(t *testing.T) {
	setupMemberProfileTestDB(t)
	user := createTestUser(t, DB.Self, "bob")
	user.Group = "前端"
	user.Github = "legacy-github"
	user.Flickr = "legacy-flickr"
	if err := user.Update(); err != nil {
		t.Fatalf("update user failed: %v", err)
	}

	profile := &MemberProfile{
		UserID:    user.Id,
		RealName:  "鲍勃",
		StudentID: "2023005",
		Group:     "Backend",
		JoinYear:  2021,
		Github:    "new-github",
	}
	if err := profile.Create(); err != nil {
		t.Fatalf("create profile failed: %v", err)
	}

	info, err := GetUserInfoByID(user.Id)
	if err != nil {
		t.Fatalf("GetUserInfoByID returned error: %v", err)
	}

	if !info.IsMuxiMember || info.MemberProfile == nil {
		t.Fatalf("expected the user to be a member, got %+v", info)
	}
	if info.StudentID != "2023005" || info.MemberProfile.RealName != "鲍勃" {
		t.Fatalf("expected profile fields to be surfaced, got %+v", info)
	}
	if info.Group != "Backend" || info.Timejoin != "2021" {
		t.Fatalf("expected group and timejoin to come from the profile, got group=%q timejoin=%q", info.Group, info.Timejoin)
	}
	// 档案里填了的覆盖，没填的保留 users 上的原值。
	if info.Github != "new-github" {
		t.Fatalf("expected profile github to win, got %q", info.Github)
	}
	if info.Flickr != "legacy-flickr" || info.Zhihu != "" {
		t.Fatalf("expected untouched legacy fields to survive, got %+v", info)
	}
	if len(info.Roles) != 2 || info.Roles[1] != RoleNameMuxiMember {
		t.Fatalf("expected roles to include muxi_member, got %v", info.Roles)
	}
}

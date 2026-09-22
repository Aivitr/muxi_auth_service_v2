package service

import (
	"errors"
	"strconv"
	"testing"

	"github.com/Muxi-X/muxi_auth_service_v2/model"
	"github.com/Muxi-X/muxi_auth_service_v2/pkg/errno"
)

func TestCheckMuxiMemberScopeSkipsRequestsWithoutTheScope(t *testing.T) {
	// 故意把 DB 置空：这个分支必须在任何数据库访问之前就返回，
	// 顺序写反的话这里会 panic 而不是静默变慢。
	oldDB := model.DB
	model.DB = nil
	t.Cleanup(func() {
		model.DB = oldDB
	})

	for _, scope := range []string{"", "openid profile", "muxi:memberX"} {
		if err := CheckMuxiMemberScope("42", scope); err != nil {
			t.Fatalf("expected scope %q to be skipped, got %v", scope, err)
		}
	}
}

func TestCheckMuxiMemberScope(t *testing.T) {
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

	memberID := strconv.FormatUint(member.Id, 10)
	outsiderID := strconv.FormatUint(outsider.Id, 10)

	for _, scope := range []string{"muxi:member", "openid muxi:member", "  muxi:member  "} {
		if err := CheckMuxiMemberScope(memberID, scope); err != nil {
			t.Fatalf("expected a member to pass scope %q, got %v", scope, err)
		}
	}

	if err := CheckMuxiMemberScope(outsiderID, "muxi:member"); !errors.Is(err, errno.ErrNotMuxiMember) {
		t.Fatalf("expected a non-member to be rejected, got %v", err)
	}

	// fail closed：真实 CAS 流程的 subject 永远是数字，历史 token 的 scope 必然是空、
	// 早在第一道守卫就放行了，所以这不会误伤真实用户。
	for _, subject := range []string{"cas:alice", "", "abc"} {
		if err := CheckMuxiMemberScope(subject, "muxi:member"); !errors.Is(err, errno.ErrNotMuxiMember) {
			t.Fatalf("expected subject %q to be rejected, got %v", subject, err)
		}
	}
}

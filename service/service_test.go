package service

import (
	"testing"

	"github.com/Muxi-X/muxi_auth_service_v2/model"
)

func TestCheckUserExistedAndNotExisted(t *testing.T) {
	db := setupCASIdentityTestDB(t)

	if err := db.Create(&model.UserModel{
		Email:    "alice@muxi.com",
		Username: "alice",
	}).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	// 登录表单只有一个字段：用户名和邮箱都要能查到同一个人。
	for _, identifier := range []string{"alice", "alice@muxi.com"} {
		got := CheckUserNotExisted(identifier)
		if got == nil || got.Username != "alice" {
			t.Fatalf("CheckUserNotExisted(%q) = %+v, want the alice account", identifier, got)
		}
	}
	if got := CheckUserNotExisted("nobody"); got != nil {
		t.Fatalf("CheckUserNotExisted(nobody) = %+v, want nil", got)
	}

	// 注册查重：用户名或邮箱任一被占用就要拦住。
	if !CheckUserExisted("alice", "fresh@muxi.com") {
		t.Fatal("an existing username should count as taken")
	}
	if !CheckUserExisted("fresh", "alice@muxi.com") {
		t.Fatal("an existing email should count as taken")
	}
	if CheckUserExisted("fresh", "fresh@muxi.com") {
		t.Fatal("an unused username/email pair should pass")
	}
}

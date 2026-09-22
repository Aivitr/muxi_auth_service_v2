package service

import (
	"context"
	"testing"

	"github.com/Muxi-X/muxi_auth_service_v2/model"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	cas "gopkg.in/cas.v2"
)

func setupCASIdentityTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open test database failed: %v", err)
	}
	db.LogMode(false)
	if err := db.AutoMigrate(&model.UserModel{}, &model.UserIdentity{}, &model.MemberProfile{}).Error; err != nil {
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

func TestResolveCASUserBindsExistingLocalUserByEmail(t *testing.T) {
	db := setupCASIdentityTestDB(t)
	existingUser := &model.UserModel{
		Email:    "alice@example.com",
		Username: "alice",
		RoleID:   3,
	}
	if err := db.Create(existingUser).Error; err != nil {
		t.Fatalf("create existing user failed: %v", err)
	}

	userID, err := (&defaultCASUserResolver{}).ResolveCASUser(context.Background(), &cas.AuthenticationResponse{
		User:       "alice.cas",
		Attributes: cas.UserAttributes{"email": []string{"alice@example.com"}},
	})
	if err != nil {
		t.Fatalf("ResolveCASUser returned error: %v", err)
	}
	if userID != existingUser.Id {
		t.Fatalf("expected existing user id %d, got %d", existingUser.Id, userID)
	}
	assertUserCount(t, db, 1)
	assertCASIdentity(t, db, "alice.cas", existingUser.Id)
}

func TestResolveCASUserBindsExistingLocalUserByUsername(t *testing.T) {
	db := setupCASIdentityTestDB(t)
	existingUser := &model.UserModel{
		Email:    "bob@example.com",
		Username: "bob",
		RoleID:   3,
	}
	if err := db.Create(existingUser).Error; err != nil {
		t.Fatalf("create existing user failed: %v", err)
	}

	userID, err := (&defaultCASUserResolver{}).ResolveCASUser(context.Background(), &cas.AuthenticationResponse{
		User: "bob",
	})
	if err != nil {
		t.Fatalf("ResolveCASUser returned error: %v", err)
	}
	if userID != existingUser.Id {
		t.Fatalf("expected existing user id %d, got %d", existingUser.Id, userID)
	}
	assertUserCount(t, db, 1)
	assertCASIdentity(t, db, "bob", existingUser.Id)
}

func TestResolveCASUserDoesNotBindExistingCASShadowUserByUsername(t *testing.T) {
	db := setupCASIdentityTestDB(t)
	shadowUser := &model.UserModel{
		Email:    "carol@example.com",
		Username: "cas_carol_shadow",
		RoleID:   3,
	}
	if err := db.Create(shadowUser).Error; err != nil {
		t.Fatalf("create shadow user failed: %v", err)
	}

	userID, err := (&defaultCASUserResolver{}).ResolveCASUser(context.Background(), &cas.AuthenticationResponse{
		User: "cas_carol_shadow",
	})
	if err != nil {
		t.Fatalf("ResolveCASUser returned error: %v", err)
	}
	if userID == shadowUser.Id {
		t.Fatalf("expected resolver not to bind shadow user id %d", shadowUser.Id)
	}
	assertUserCount(t, db, 2)
	assertCASIdentity(t, db, "cas_carol_shadow", userID)
}

func assertUserCount(t *testing.T, db *gorm.DB, want int) {
	t.Helper()

	var count int
	if err := db.Model(&model.UserModel{}).Count(&count).Error; err != nil {
		t.Fatalf("count users failed: %v", err)
	}
	if count != want {
		t.Fatalf("expected %d users, got %d", want, count)
	}
}

func assertCASIdentity(t *testing.T, db *gorm.DB, providerSubject string, wantUserID uint64) {
	t.Helper()

	identity := &model.UserIdentity{}
	err := db.Where("provider = ? AND provider_subject = ?", casIdentityProvider, providerSubject).First(identity).Error
	if err != nil {
		t.Fatalf("get CAS identity failed: %v", err)
	}
	if identity.UserID != wantUserID {
		t.Fatalf("expected identity user id %d, got %d", wantUserID, identity.UserID)
	}
}

func TestBuildCASLocalUsername(t *testing.T) {
	got := BuildCASLocalUsername(" Alice.Z ")
	if got != "cas_alice_z_50f08eb7" {
		t.Fatalf("unexpected local username: %s", got)
	}
}

func TestBuildCASLocalUsernameHandlesNonASCII(t *testing.T) {
	got := BuildCASLocalUsername("木犀")
	if got != "cas_user_05745470" {
		t.Fatalf("unexpected local username: %s", got)
	}
}

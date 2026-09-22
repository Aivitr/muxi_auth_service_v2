package model

import (
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/Muxi-X/muxi_auth_service_v2/pkg/constvar"
	"github.com/Muxi-X/muxi_auth_service_v2/pkg/logx"
	"github.com/Muxi-X/muxi_auth_service_v2/util/captcha"
	"github.com/ShiinaOrez/GoSecurity/security"
)

// User represents a registered user.
type UserModel struct {
	BaseModel
	Email        string `json:"email" column:"email" binding:"required"`
	Birthday     string `json:"birthday" column:"birthday"`
	Hometown     string `json:"hometown" column:"hometown"`
	Group        string `json:"group" column:"group"`
	Timejoin     string `json:"timejoin" column:"timejoin"`
	Timeleft     string `json:"timeleft" column:"timeleft"`
	Username     string `json:"username" column:"username" binding:"required"`
	PasswordHash string `json:"password_hash" column:"password_hash" binding:"required"`
	RoleID       uint64 `json:"role_id" column:"role_id"`
	Left         bool   `json:"left" column:"left"`
	ResetT       string `json:"reset_t" column:"reste_t"`
	Info         string `json:"info" column:"info"`
	AvatarURL    string `json:"avatar_url" column:"avatar_url"`
	PersonalBlog string `json:"personal_blog" column:"personal_blog"`
	Github       string `json:"github" column:"github"`
	Flickr       string `json:"flickr" column:"flickr"`
	Weibo        string `json:"weibo" column:"weibo"`
	Zhihu        string `json:"zhihu" column:"zhihu"`
}

func (c *UserModel) TableName() string {
	return "users"
}

// Create creates a new user account.
func (u *UserModel) Create() error {
	return DB.Self.Create(&u).Error
}

// DeleteUser deletes the user by the user identifier.
func DeleteUser(id uint64) error {
	user := UserModel{}
	user.BaseModel.Id = id
	return DB.Self.Delete(&user).Error
}

// Update updates an user account information.
func (u *UserModel) Update() error {
	return DB.Self.Save(u).Error
}

// role_id 取值来自 db.sql 里 roles 表的种子数据。
const (
	RoleIDModerator = 1
	RoleIDAdmin     = 2
	RoleIDUser      = 3
)

// 对下游暴露的角色名。
const (
	RoleNameUser       = "user"
	RoleNameModerator  = "moderator"
	RoleNameAdmin      = "admin"
	RoleNameMuxiMember = "muxi_member"
)

func (u *UserModel) IsAdmin() bool {
	return u.RoleID == RoleIDAdmin
}

// BuildUserRoles 返回值永不为 nil：json.Marshal(nil slice) 产出 null，前端 roles.length 会直接炸。
func BuildUserRoles(user *UserModel, isMuxiMember bool) []string {
	roles := []string{RoleNameUser}

	if isMuxiMember {
		roles = append(roles, RoleNameMuxiMember)
	}

	switch user.RoleID {
	case RoleIDAdmin:
		roles = append(roles, RoleNameAdmin)
	case RoleIDModerator:
		roles = append(roles, RoleNameModerator)
	}

	return roles
}

// HasAdminAccess 是「谁能进后台」的唯一定义处，middleware 只做委托。
// 放在 model 是为了避免循环 import：middleware 已经 import model，反向会成环。
func HasAdminAccess(user *UserModel) bool {
	if user == nil {
		return false
	}
	if user.IsAdmin() {
		return true
	}

	role, err := GetRoleByID(user.RoleID)
	if err != nil || role == nil {
		return false
	}

	return role.Permissions&constvar.PermissionOAuthClientManage != 0
}

func (user *UserModel) CheckPassword(passwordBase64 string) bool {
	password, err := UserPasswordDecoder(passwordBase64)
	if err != nil {
		return false
	}
	return security.CheckPasswordHash(password, user.PasswordHash)
}

func UserPasswordDecoder(passwordBase64 string) (string, error) {
	passwordBytes, err := base64.StdEncoding.DecodeString(passwordBase64)
	if err != nil {
		return "", err
	}
	return string(passwordBytes), err
}

func GeneratePasswordHash(password string) string {
	return security.GeneratePasswordHash(password)
}

func GetUserByID(id uint64) (*UserModel, error) {
	user := &UserModel{}
	d := DB.Self.Where("id = ?", id).First(&user)
	return user, d.Error
}

func GetUserByEmail(email string) (*UserModel, error) {
	user := &UserModel{}
	d := DB.Self.Where("email = ?", email).First(&user)
	return user, d.Error
}

func GetUserByUsername(username string) (*UserModel, error) {
	user := &UserModel{}
	d := DB.Self.Where("username = ?", username).First(&user)
	return user, d.Error
}

func GetEmailByUsername(username string) (string, error) {
	user := &UserModel{}
	d := DB.Self.Select("email").Where("username = ?", username).First(&user)
	return user.Email, d.Error
}

func (user *UserModel) VerifyCaptcha(newCap string) bool {
	oldCap, err := captcha.ResolveCaptchaToken(user.ResetT)
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	fmt.Println(oldCap, newCap)
	return oldCap == newCap
}

func GetUserInfoByID(id uint64) (*UserInfo, error) {
	user, err := GetUserByID(id)
	if err != nil {
		return nil, err
	}

	// fail-soft：member_profiles 没建好时降级成非成员，而不是让全站 /auth/api/user 一起 500。
	profile, err := GetMemberProfileByUserID(id)
	if err != nil {
		logx.Error("Failed to load member profile", "user_id", id, "error", err)
		profile = nil
	}

	return BuildUserInfo(user, profile), nil
}

// BuildUserInfo 的 profile 为 nil 表示不是木犀成员。
func BuildUserInfo(user *UserModel, profile *MemberProfile) *UserInfo {
	info := &UserInfo{
		UserID:       user.Id,
		Email:        user.Email,
		Birthday:     user.Birthday,
		Hometown:     user.Hometown,
		Group:        user.Group,
		Timejoin:     user.Timejoin,
		Timeleft:     user.Timeleft,
		Username:     user.Username,
		RoleID:       user.RoleID,
		Left:         user.Left,
		Info:         user.Info,
		AvatarURL:    user.AvatarURL,
		PersonalBlog: user.PersonalBlog,
		Github:       user.Github,
		Flickr:       user.Flickr,
		Weibo:        user.Weibo,
		Zhihu:        user.Zhihu,
	}

	if profile != nil {
		info.IsMuxiMember = true
		info.MemberProfile = profile
		info.StudentID = profile.StudentID
		info.Group = preferNonEmpty(profile.Group, info.Group)
		info.PersonalBlog = preferNonEmpty(profile.PersonalBlog, info.PersonalBlog)
		info.Github = preferNonEmpty(profile.Github, info.Github)
		info.Zhihu = preferNonEmpty(profile.Zhihu, info.Zhihu)
		if profile.JoinYear != 0 {
			info.Timejoin = strconv.Itoa(profile.JoinYear)
		}
	}

	info.Roles = BuildUserRoles(user, info.IsMuxiMember)

	return info
}

// preferNonEmpty 避免迁移把下游本来能看到的社交链接清空：档案里没填就保留 users 上的原值。
func preferNonEmpty(profileValue, legacyValue string) string {
	if profileValue != "" {
		return profileValue
	}

	return legacyValue
}

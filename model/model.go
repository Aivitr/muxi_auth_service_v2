package model

import (
	"sync"
)

type BaseModel struct {
	Id uint64 `gorm:"primary_key;AUTO_INCREMENT;column:id" json:"-"`
	// CreatedAt time.Time `gorm:"column:createdAt" json:"-"`
	// UpdatedAt time.Time `gorm:"column:updatedAt" json:"-"`
	// DeletedAt *time.Time `gorm:"column:deletedAt" sql:"index" json:"-"`
}

type UserList struct {
	Lock  *sync.Mutex
	IdMap map[uint64]*UserModel
}

// Token represents a JSON web token.
type Token struct {
	Token string `json:"token"`
}

// UserInfo 不对应任何表，不要再用 Table("users").First(&info) 查它：
// gorm 会去 select 不存在的列并直接报错。
type UserInfo struct {
	UserID        uint64         `json:"user_id"`
	StudentID     string         `json:"student_id"`
	IsMuxiMember  bool           `json:"is_muxi_member"`
	Roles         []string       `json:"roles"`
	MemberProfile *MemberProfile `json:"member_profile"`

	// 以下为兼容存量下游保留，成员身份下由 member_profiles 回填。
	Email        string `json:"email"`
	Birthday     string `json:"birthday"`
	Hometown     string `json:"hometown"`
	Group        string `json:"group"`
	Timejoin     string `json:"timejoin"`
	Timeleft     string `json:"timeleft"`
	Username     string `json:"username"`
	RoleID       uint64 `json:"role_id"`
	Left         bool   `json:"left"`
	Info         string `json:"info"`
	AvatarURL    string `json:"avatar_url"`
	PersonalBlog string `json:"personal_blog"`
	Github       string `json:"github"`
	Flickr       string `json:"flickr"`
	Weibo        string `json:"weibo"`
	Zhihu        string `json:"zhihu"`
}

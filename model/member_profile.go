package model

import (
	"strconv"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
)

// searchUsersMaxLimit 限制搜索返回条数，避免这个接口被当成全量导出。
const searchUsersMaxLimit = 20

// MemberProfile 是木犀团队成员的人事档案，只有被管理员授权过的用户才会有记录。
type MemberProfile struct {
	BaseModel
	UserID       uint64    `gorm:"column:user_id" json:"user_id"`
	RealName     string    `gorm:"column:real_name" json:"real_name"`
	StudentID    string    `gorm:"column:student_id" json:"student_id"`
	Group        string    `gorm:"column:group" json:"group"`
	JoinYear     int       `gorm:"column:join_year" json:"join_year"`
	PersonalBlog string    `gorm:"column:personal_blog" json:"personal_blog"`
	Github       string    `gorm:"column:github" json:"github"`
	Zhihu        string    `gorm:"column:zhihu" json:"zhihu"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (m *MemberProfile) TableName() string {
	return "member_profiles"
}

func (m *MemberProfile) Create() error {
	return DB.Self.Create(m).Error
}

func (m *MemberProfile) Update() error {
	return DB.Self.Save(m).Error
}

func DeleteMemberProfile(userID uint64) error {
	return DB.Self.Where("user_id = ?", userID).Delete(&MemberProfile{}).Error
}

// GetMemberProfileByUserID 在非成员时返回 (nil, nil)。
// “不是成员”是正常状态：当成 error 往上抛会让所有非成员的 /auth/api/user 变成 500。
func GetMemberProfileByUserID(userID uint64) (*MemberProfile, error) {
	profile := &MemberProfile{}
	err := DB.Self.Where("user_id = ?", userID).First(profile).Error
	if err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}

	return profile, nil
}

func IsMuxiMember(userID uint64) (bool, error) {
	profile, err := GetMemberProfileByUserID(userID)
	if err != nil {
		return false, err
	}

	return profile != nil, nil
}

// IsStudentIDTaken 在应用层兜学号唯一性：DB 上只有普通索引，
// 因为迁移后会有多行空学号，唯一索引会直接冲突。空学号不参与约束。
func IsStudentIDTaken(studentID string, excludeUserID uint64) (bool, error) {
	studentID = strings.TrimSpace(studentID)
	if studentID == "" {
		return false, nil
	}

	var count uint64
	err := DB.Self.Model(&MemberProfile{}).
		Where("student_id = ? AND user_id <> ?", studentID, excludeUserID).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// MemberListItem 的列名与字段名对不上，必须显式标注 gorm column，
// 否则 Scan 会静默填零值（列表全空而且不报错）。
type MemberListItem struct {
	UserID       uint64    `gorm:"column:user_id" json:"user_id"`
	Username     string    `gorm:"column:username" json:"username"`
	Email        string    `gorm:"column:email" json:"email"`
	AvatarURL    string    `gorm:"column:avatar_url" json:"avatar_url"`
	RealName     string    `gorm:"column:real_name" json:"real_name"`
	StudentID    string    `gorm:"column:student_id" json:"student_id"`
	Group        string    `gorm:"column:group" json:"group"`
	JoinYear     int       `gorm:"column:join_year" json:"join_year"`
	PersonalBlog string    `gorm:"column:personal_blog" json:"personal_blog"`
	Github       string    `gorm:"column:github" json:"github"`
	Zhihu        string    `gorm:"column:zhihu" json:"zhihu"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
}

func ListMemberProfiles(offset, limit int, group, keyword string) ([]*MemberListItem, uint64, error) {
	query := DB.Self.Table("member_profiles AS mp").
		Joins("JOIN users AS u ON u.id = mp.user_id")

	// group 是 MySQL 保留字，且 users 与 member_profiles 都有这一列，
	// 所以必须写成 mp.`group`（带表前缀 + 反引号），两个方言都认。
	if group != "" {
		query = query.Where("mp.`group` = ?", group)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where(
			"mp.real_name LIKE ? OR mp.student_id LIKE ? OR u.username LIKE ? OR u.email LIKE ?",
			like, like, like, like,
		)
	}

	var total uint64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]*MemberListItem, 0, limit)
	err := query.
		Select("mp.user_id, u.username, u.email, u.avatar_url, mp.real_name, mp.student_id, " +
			"mp.`group` AS `group`, COALESCE(mp.join_year, 0) AS join_year, " +
			"mp.personal_blog, mp.github, mp.zhihu, mp.created_at").
		Order("mp.id DESC").
		Offset(offset).Limit(limit).
		Scan(&items).Error
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

type UserSearchItem struct {
	UserID       uint64 `gorm:"column:user_id" json:"user_id"`
	Username     string `gorm:"column:username" json:"username"`
	Email        string `gorm:"column:email" json:"email"`
	AvatarURL    string `gorm:"column:avatar_url" json:"avatar_url"`
	RealName     string `gorm:"column:real_name" json:"real_name"`
	StudentID    string `gorm:"column:student_id" json:"student_id"`
	IsMuxiMember bool   `gorm:"column:is_muxi_member" json:"is_muxi_member"`
}

func SearchUsers(keyword string, limit int) ([]*UserSearchItem, error) {
	keyword = strings.TrimSpace(keyword)
	items := make([]*UserSearchItem, 0)
	if keyword == "" {
		return items, nil
	}
	if limit <= 0 || limit > searchUsersMaxLimit {
		limit = searchUsersMaxLimit
	}

	like := "%" + keyword + "%"
	err := DB.Self.Table("users AS u").
		Joins("LEFT JOIN member_profiles AS mp ON mp.user_id = u.id").
		Select("u.id AS user_id, u.username, u.email, u.avatar_url, "+
			"COALESCE(mp.real_name, '') AS real_name, COALESCE(mp.student_id, '') AS student_id, "+
			"CASE WHEN mp.user_id IS NULL THEN 0 ELSE 1 END AS is_muxi_member").
		Where("u.username LIKE ? OR u.email LIKE ? OR mp.student_id LIKE ?", like, like, like).
		Order("u.id ASC").
		Limit(limit).
		Scan(&items).Error
	if err != nil {
		return nil, err
	}

	return items, nil
}

// RevokeUserOAuthTokens 只断掉 refresh 续期。已签发的 JWT access token 是自包含的，
// 在剩余有效期内仍能通过本地验签使用，要即时失效只能靠下游回查 /auth/api/user。
func RevokeUserOAuthTokens(userID uint64) error {
	// oauth2_token 由依赖在服务启动时建表，测试或没启动过的环境里不存在。
	if !DB.Self.HasTable("oauth2_token") {
		return nil
	}

	return DB.Self.Exec(
		"DELETE FROM oauth2_token WHERE JSON_VALID(data) = 1 "+
			"AND JSON_UNQUOTE(JSON_EXTRACT(data, '$.UserID')) = ?",
		strconv.FormatUint(userID, 10),
	).Error
}

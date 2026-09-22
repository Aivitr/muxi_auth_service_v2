package member

import (
	"strconv"
	"strings"

	"github.com/Muxi-X/muxi_auth_service_v2/handler"
	"github.com/Muxi-X/muxi_auth_service_v2/model"
	"github.com/Muxi-X/muxi_auth_service_v2/pkg/constvar"
	"github.com/Muxi-X/muxi_auth_service_v2/pkg/errno"
	"github.com/Muxi-X/muxi_auth_service_v2/pkg/logx"

	"github.com/gin-gonic/gin"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// ponytail: PUT 下零值表示「不改这一项」，所以改不回空串（比如清空 github）。
// 真需要清空时把字段换成指针，现在没这个需求。
type MemberRequest struct {
	UserID       uint64 `json:"user_id"`
	RealName     string `json:"real_name"`
	StudentID    string `json:"student_id"`
	Group        string `json:"group"`
	JoinYear     int    `json:"join_year"`
	PersonalBlog string `json:"personal_blog"`
	Github       string `json:"github"`
	Zhihu        string `json:"zhihu"`
}

type MemberListResponse struct {
	List     []*model.MemberListItem `json:"list"`
	Total    uint64                  `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
}

type UserSearchResponse struct {
	List []*model.UserSearchItem `json:"list"`
}

func List(c *gin.Context) {
	page := parsePositiveInt(c.Query("page"), 1)
	pageSize := min(parsePositiveInt(c.Query("page_size"), defaultPageSize), maxPageSize)

	items, total, err := model.ListMemberProfiles(
		(page-1)*pageSize, pageSize, c.Query("group"), c.Query("query"))
	if err != nil {
		handler.SendError(c, err, nil, err.Error())
		return
	}

	handler.SendResponse(c, nil, MemberListResponse{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func Create(c *gin.Context) {
	var rq MemberRequest
	if err := c.BindJSON(&rq); err != nil {
		handler.SendBadRequest(c, errno.ErrBadRequest, nil, err.Error())
		return
	}

	if rq.UserID == 0 {
		handler.SendBadRequest(c, errno.ErrBadRequest, nil, "user_id is required")
		return
	}
	if !constvar.IsValidMemberGroup(rq.Group) {
		handler.SendBadRequest(c, errno.ErrInvalidMemberGroup, nil, rq.Group)
		return
	}

	if _, err := model.GetUserByID(rq.UserID); err != nil {
		handler.SendNotFound(c, errno.ErrUserNotFound, nil, "user does not exist")
		return
	}

	existing, err := model.GetMemberProfileByUserID(rq.UserID)
	if err != nil {
		handler.SendError(c, err, nil, err.Error())
		return
	}
	if existing != nil {
		handler.SendBadRequest(c, errno.ErrMemberProfileExisted, nil, "")
		return
	}

	studentID := strings.TrimSpace(rq.StudentID)
	if taken, err := model.IsStudentIDTaken(studentID, rq.UserID); err != nil {
		handler.SendError(c, err, nil, err.Error())
		return
	} else if taken {
		handler.SendBadRequest(c, errno.ErrStudentIDExisted, nil, studentID)
		return
	}

	profile := &model.MemberProfile{
		UserID:       rq.UserID,
		RealName:     strings.TrimSpace(rq.RealName),
		StudentID:    studentID,
		Group:        rq.Group,
		JoinYear:     rq.JoinYear,
		PersonalBlog: rq.PersonalBlog,
		Github:       rq.Github,
		Zhihu:        rq.Zhihu,
	}
	if err := profile.Create(); err != nil {
		handler.SendError(c, err, nil, err.Error())
		return
	}

	handler.SendResponse(c, nil, profile)
}

func Update(c *gin.Context) {
	userID, ok := parseUserIDParam(c)
	if !ok {
		return
	}

	var rq MemberRequest
	if err := c.BindJSON(&rq); err != nil {
		handler.SendBadRequest(c, errno.ErrBadRequest, nil, err.Error())
		return
	}
	if rq.Group != "" && !constvar.IsValidMemberGroup(rq.Group) {
		handler.SendBadRequest(c, errno.ErrInvalidMemberGroup, nil, rq.Group)
		return
	}

	profile, err := model.GetMemberProfileByUserID(userID)
	if err != nil {
		handler.SendError(c, err, nil, err.Error())
		return
	}
	if profile == nil {
		handler.SendNotFound(c, errno.ErrMemberProfileNotFound, nil, "")
		return
	}

	studentID := strings.TrimSpace(rq.StudentID)
	if studentID != "" {
		taken, err := model.IsStudentIDTaken(studentID, userID)
		if err != nil {
			handler.SendError(c, err, nil, err.Error())
			return
		}
		if taken {
			handler.SendBadRequest(c, errno.ErrStudentIDExisted, nil, studentID)
			return
		}
		profile.StudentID = studentID
	}

	if realName := strings.TrimSpace(rq.RealName); realName != "" {
		profile.RealName = realName
	}
	if rq.Group != "" {
		profile.Group = rq.Group
	}
	if rq.JoinYear != 0 {
		profile.JoinYear = rq.JoinYear
	}
	if rq.PersonalBlog != "" {
		profile.PersonalBlog = rq.PersonalBlog
	}
	if rq.Github != "" {
		profile.Github = rq.Github
	}
	if rq.Zhihu != "" {
		profile.Zhihu = rq.Zhihu
	}

	if err := profile.Update(); err != nil {
		handler.SendError(c, err, nil, err.Error())
		return
	}

	handler.SendResponse(c, nil, profile)
}

func Delete(c *gin.Context) {
	userID, ok := parseUserIDParam(c)
	if !ok {
		return
	}

	profile, err := model.GetMemberProfileByUserID(userID)
	if err != nil {
		handler.SendError(c, err, nil, err.Error())
		return
	}
	if profile == nil {
		handler.SendNotFound(c, errno.ErrMemberProfileNotFound, nil, "")
		return
	}

	if err := model.DeleteMemberProfile(userID); err != nil {
		handler.SendError(c, err, nil, err.Error())
		return
	}

	// 撤销失败不回滚：档案已经删掉了，让管理员重试只会拿到 404。
	if err := model.RevokeUserOAuthTokens(userID); err != nil {
		logx.Error("Failed to revoke oauth tokens of removed member", "user_id", userID, "error", err)
	}

	handler.SendResponse(c, nil, nil)
}

func SearchUsers(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("q"))
	if keyword == "" {
		handler.SendBadRequest(c, errno.ErrBadRequest, nil, "q is required")
		return
	}

	items, err := model.SearchUsers(keyword, 0)
	if err != nil {
		handler.SendError(c, err, nil, err.Error())
		return
	}

	handler.SendResponse(c, nil, UserSearchResponse{List: items})
}

func parseUserIDParam(c *gin.Context) (uint64, bool) {
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 64)
	if err != nil {
		handler.SendBadRequest(c, errno.ErrBadRequest, nil, "invalid user_id")
		return 0, false
	}

	return userID, true
}

func parsePositiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}

	return value
}

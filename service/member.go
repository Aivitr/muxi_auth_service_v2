package service

import (
	"strconv"

	"github.com/Muxi-X/muxi_auth_service_v2/model"
	"github.com/Muxi-X/muxi_auth_service_v2/pkg/constvar"
	"github.com/Muxi-X/muxi_auth_service_v2/pkg/errno"
	"github.com/Muxi-X/muxi_auth_service_v2/pkg/logx"
)

// CheckMuxiMemberScope 在签发授权码之前，拦掉申请了 muxi:member 却不是木犀成员的人。
// 参数吃 string 是为了让本地账号密码链路和 CAS 链路共用一份实现。
//
// 三个分支的顺序不能换：先看 scope 再碰库。既有 CAS 测试里 model.DB.Self 是 nil，
// 无条件查库会直接 panic。
func CheckMuxiMemberScope(userID, rawScope string) error {
	if !constvar.HasScope(rawScope, constvar.ScopeMuxiMember) {
		return nil
	}

	id, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		// fail closed。真实 CAS 流程的 subject 永远是数字，"cas:<username>" 只见于测试，
		// 而历史 token 的 scope 必然是空、上面第一行就放行了，所以对真实用户零影响。
		return errno.ErrNotMuxiMember
	}

	isMember, err := model.IsMuxiMember(id)
	if err != nil {
		// 同样 fail closed：查不出来就不放行。
		logx.Error("Failed to check muxi membership", "user_id", id, "error", err)
		return errno.ErrNotMuxiMember
	}
	if !isMember {
		return errno.ErrNotMuxiMember
	}

	return nil
}

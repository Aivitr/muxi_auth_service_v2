package service

import "github.com/Muxi-X/muxi_auth_service_v2/model"

// CheckUserExisted 用于注册查重：用户名或邮箱任一被占用就算已存在。
func CheckUserExisted(username, email string) bool {
	if _, err := model.GetUserByUsername(username); err == nil {
		return true
	}

	_, err := model.GetUserByEmail(email)
	return err == nil
}

// CheckUserNotExisted 按用户名或邮箱找用户：登录表单只有一个字段，
// 用户填进来的可能是用户名，也可能是邮箱，所以同一个字符串查两列。
func CheckUserNotExisted(identifier string) *model.UserModel {
	if user, err := model.GetUserByUsername(identifier); err == nil {
		return user
	}

	user, err := model.GetUserByEmail(identifier)
	if err != nil {
		return nil
	}

	return user
}

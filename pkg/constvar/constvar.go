package constvar

import (
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	DefaultLimit                = 50
	PermissionOAuthClientManage = 1

	// 木犀团队成员所属组别，仅用于管理端写入校验
	MemberGroupFrontend  = "Frontend"
	MemberGroupBackend   = "Backend"
	MemberGroupDesign    = "Design"
	MemberGroupProduct   = "Product"
	MemberGroupOperation = "Operation"

	// ScopeMuxiMember 是内部业务系统申请“仅木犀成员可访问”时携带的 OAuth scope
	ScopeMuxiMember = "muxi:member"
	EmailTemp       = `<table border="0" cellspacing="0" cellpadding="0" style="max-width: 600px;">
	<tbody>
		<tr height="16"></tr>
		<tr>
			<td>
				<table bgcolor="#4184F3" width="100%" border="0" cellspacing="0" cellpadding="0" style="min-width: 332px; max-width: 600px; border: 1px solid #E0E0E0; border-bottom: 0; border-top-left-radius: 3px; border-top-right-radius: 3px;">
					<tbody>
						<tr>
							<td height="48px" colspan="3"></td>
						</tr>
						<tr>
							<td width="32px"></td>
							<td style="font-family: Roboto-Regular,Helvetica,Arial,sans-serif; font-size: 24px; color: #FFFFFF; line-height: 1.25;">MuxiStudio: 通行证 验证码</td>
							<td width="32px"></td>
						</tr>
						<tr>
							<td height="18px" colspan="3"></td>
						</tr>
					</tbody>
				</table>
			</td>
		</tr>
		<tr>
			<td>
				<table bgcolor="#FAFAFA" width="100%" border="0" cellspacing="0" cellpadding="0" style="min-width: 332px; max-width: 600px; border: 1px solid #F0F0F0; border-bottom: 1px solid #C0C0C0; border-top: 0; border-bottom-left-radius: 3px; border-bottom-right-radius: 3px;">
					<tbody>
						<tr height="16px">
							<td width="32px" rowspan="3"></td>
							<td></td>
							<td width="32px" rowspan="3"></td>
						</tr>
						<tr>
							<td>
								<p>尊敬的 木犀通行证 用户：</p>
								<p>我们收到了一项请求，要求通过您的电子邮件地址访问您的 木犀通行证 帐号，以进行密码重置操作 
									<span style="color: #659CEF" dir="ltr">
										<a href="mailto:YourEmailAddress" rel="noopener" target="_blank">YourEmailAddress</a>
										</span>。您的 木犀通行证 验证码为：
									</p>
									<div style="text-align: center;">
										<p dir="ltr">
											<strong style="text-align: center; font-size: 24px; font-weight: bold;">TheCaptcha</strong>
										</p>
									</div>
									<p>如果您并未请求此验证码，则可能是他人正在尝试访问以下 木犀通行证 帐号：
										<span style="color: #659CEF" dir="ltr">
											<a href="mailto:YourEmailAddress" rel="noopener" target="_blank">YourEmailAddress
											</a>
										</span>。
										<strong>请勿将此验证码转发给或提供给任何人。</strong>
									</p>
									<p>您之所以会收到此邮件，是因为此电子邮件地址已被设为 木犀通行证 帐号 <span style="color: #659CEF">
										<a href="mailto:YourEmailAddress" rel="noopener" target="_blank">YourEmailAddress</a>
										</span> 的邮箱
									</p>
									<p>此致</p>
									<p>MuxiStudio 木犀团队敬上</p>
								</td>
							</tr>
							<tr height="32px"></tr>
						</tbody>
					</table>
				</td>
			</tr>
			<tr height="16"></tr>
			<tr>
				<td style="max-width: 600px; font-family: Roboto-Regular,Helvetica,Arial,sans-serif; font-size: 10px; color: #BCBCBC; line-height: 1.5;">
				</td>
			</tr>
			<tr>
				<td>此电子邮件地址无法接收回复。如需更多信息，请访问 
					<a href="https://www.muxixyz.com" style="text-decoration: none; color: #4d90fe;" rel="noopener" target="_blank">木犀团队官网
					</a>。
					<br>Muxi Studio, CCNU, Wuhan HuBei, China 
					<table style="font-family: Roboto-Regular,Helvetica,Arial,sans-serif; font-size: 10px; color: #666666; line-height: 18px; padding-bottom: 10px">						
					</table>
				</td>
			</tr>
		</tbody>
	</table>`
)

var (
	TestRouter *gin.Engine
	Token      string
)

var MemberGroups = []string{
	MemberGroupFrontend,
	MemberGroupBackend,
	MemberGroupDesign,
	MemberGroupProduct,
	MemberGroupOperation,
}

// IsValidMemberGroup 只用于管理端写入校验。
// 历史 users.group 里有“前端”“Android”这类自由文本，读路径不能拿白名单过滤。
func IsValidMemberGroup(group string) bool {
	return slices.Contains(MemberGroups, group)
}

// HasScope 按 OAuth 2.0 的空格分隔切分，顺带兼容逗号。
func HasScope(rawScope, target string) bool {
	fields := strings.FieldsFunc(rawScope, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == ','
	})

	return slices.Contains(fields, target)
}

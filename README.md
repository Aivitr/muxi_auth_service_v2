## 木犀通行证 v2

![travis-ci](https://travis-ci.org/Muxi-X/muxi_auth_service_v2.svg?branch=master)

### 简介

木犀通行证旨在构建统一的木犀内外门户，本仓库为原Python版本基础之上修改而来，使用Go语言重构。

主要依赖：gin + gorm + viper + lexkong/log

支持Go语言版本： Golang 1.12 及以上

### 构建和运行 Build and run

```
make && ./main
```

### 测试 Testing

```
make test
```

### 包管理

go module

```shell
go mod tidy
```

### APIs

详见 [文档](./api.yaml)

### OAuth

采用 OAuth2.0 标准，使用**授权码模式**进行认证。

#### 登录

授权码模式，客户端要求是前后端分离的应用。

流程：

```
Frontend              Backend                 Auth server

    +                     +                       +
    |                     |                       |
    |                     |                       |
    |                     |                       |
    |           1) login and auth                 |
    |  +--------------------------------------->  |
    |                     |                       |
    |                     |                       |
    |                     |                       |
    |           2) return auth code               |
    |  <------------------+--------------------+  |
    |                     |                       |
    |                     |                       |
    |     3) login        |                       |
    |  +--------------->  |                       |
    |                     |                       |
    |                     | 4) get access token   |
    |                     | +-------------------> |
    |                     |                       |
    |                     |                       |
    |                     | 5)return access token |
    |                     | <-------------------+ |
    |                     |                       |
    | 6)login successfully|                       |
    | <-----------------+ |                       |
    |                     |                       |
    |                     |                       |
    |                     |                       |
    +                     +                       +
```

1. 客户端前端向 Auth 服务器请求 `auth code`，通过 `.../oauth/auth` [API](#登录--获取授权码)
2. 登录成功后，Auth 服务器返回 auth code；
3. 前端向后端请求登录；
4. 后端向 Auth 服务器请求 access token，通过 `.../oauth/token` [API](#get-access-token)
5. 验证通过，Auth 服务器返回 access token；
6. 后端生成 token（客户端应用所用的），返回给前端；
7. 登录成功。


#### 获取用户信息

使用 `access token`，通过 `.../auth/api/user` API 获取用户信息。

响应里的 `is_muxi_member`、`roles`、`member_profile` 是判断「这个账号是不是木犀团队成员」的入口。`roles` 永远是非空数组，`member_profile` 在非成员时是 `null`。

**下游内部系统判断成员身份必须回查这个接口，不能只看 token。** OAuth scope（见下节）只在授权那一刻拦截一次，已签发的 access token 是自包含的 JWT，业务方本地验签不查库——成员被移除后，他手里的 token 在剩余有效期内（最长 30 天）仍然可用。

#### 木犀成员身份

基础账号（`users`）和木犀成员人事档案（`member_profiles`）已经完全解耦：全校学生都有账号，只有被管理员授权过的人在 `member_profiles` 里有一条记录。

授权与撤销全部走管理端接口，需要本地管理员账号的 `access token`：

| 接口 | 说明 |
|---|---|
| `GET /auth/api/admin/members` | 分页列表，支持 `group` 过滤与 `query` 模糊搜索 |
| `POST /auth/api/admin/members` | 授权，body 传 `user_id` / `group`（必填）等 |
| `PUT /auth/api/admin/members/:user_id` | 修改档案，字段可选 |
| `DELETE /auth/api/admin/members/:user_id` | 撤销成员身份，同时撤销该用户的 OAuth token |
| `GET /auth/api/admin/users/search?q=` | 按用户名/邮箱/学号搜账号，用于授权时选人 |

组别只接受 `Frontend` / `Backend` / `Design` / `Product` / `Operation`。历史数据里存在「前端」「Android」这类自由文本，读路径不会拿白名单过滤，只有写入才校验。

学号唯一性在应用层校验，DB 上只有普通索引：迁移后会有大量空学号，唯一索引会直接冲突。空学号不参与约束。

撤销成员时调用 `DELETE` 会删掉 `oauth2_token` 里该用户的记录，效果是 refresh token 立即失效、无法续期。**已经签发出去的 access token 不会立刻失效**，要即时失效只能靠下游回查 `/auth/api/user`。

#### OAuth scope：只放木犀成员进来

内部业务系统要求「只有木犀团队成员能访问」时，在授权请求里带上 `scope=muxi:member`：

- 本地账号密码链路：`POST /auth/api/oauth?scope=muxi:member`，**scope 必须放 query，放 body 读不到**（那几个参数走 `r.FormValue`，JSON body 不会被解析）
- CAS 链路：前端拼 CAS `service` URL 时要带上 `scope`，否则走 CAS 一键登录会绕过拦截

非成员请求会被拒（HTTP 403 + 错误码 40008）。这是 `/auth/api/oauth` 唯一一个走非 200 的分支，其他失败都是 HTTP 200 + 业务错误码，前端要单独处理。

不带 `scope` 的请求完全不受影响，对外应用照常拿到授权码。

**这个机制只在授权时拦一次，做不到每次业务请求鉴权。** scope 写不进 JWT claim，`/oauth/token` 的响应里也不含 scope，所以下游拿不到它。真正的持续鉴权得靠回查 `/auth/api/user`。

#### 部署顺序

**先执行 `migrations/20260922_create_member_profiles.sql` 建表，再发新版应用。**

反过来的话新代码读不到 `member_profiles`：虽然做了 fail-soft 不会 500，但所有用户的 `is_muxi_member` 会静默变成 `false`。

历史成员数据迁移分两步，两个脚本都由人工在目标环境执行：

1. `migrations/20260922_preview_migrate_members.sql` —— 只读，打印候选总数、`group` 取值分布、`timejoin` 样本、离职成员数等，用来确认数据形状和该不该一并迁入 `left = 1` 的人
2. `migrations/20260922_migrate_members.sql` —— 实际写入，可重复执行。**迁移后 `real_name` 和 `student_id` 全是空的**（`users` 表根本没有这两列），脚本结尾会打出待补录清单，需要管理员在后台逐个补齐

#### 更新 access token

客户端通过 [refresh token API](#fresh-access-token) `refresh token`，进行 `access token` 的更新。

#### 客户端注册

客户端注册接口（`.../oauth/store`）已恢复为管理员接口。需要新增 OAuth 客户端时，应由管理员审核业务域名与回调地址后，携带管理员 OAuth access token 创建客户端信息。

#### OAuth APIs

##### 登录 & 获取授权码

| Path | Method | Header |
| ---  | ---    | ---    |
| /auth/api/oauth | POST | - |

Query Param:
```
    response_type: code （固定字段）
    client_id:
    token_exp: token过期时间，可选
```

Body Data:
```json
{
    "username": "",
    "password": "" // 密码（base64）
}
```

Response:
```json
{
    "code": "",
    "expired": 0, // 过期时间（s）
}
```

##### Get access token

| Path | Method | Header |
| ---  | ---    | ---    |
| /auth/api/oauth/token | POST | - |

Query Param:
```
    grant_type: authorization_code （固定字段）
    response_type: token （固定字段）
    client_id:
```

Body Data (Forms):
```
    client_secret:
    code: 授权码
```

Response Data:
```json
{
    "access_token": "",
    "access_expired": 0, // 过期时间（s）
    "refresh_token": "",
    "refresh_expired": 0 // 过期时间（s）
}
```

##### Refresh access token

| Path | Method | Header |
| ---  | ---    | ---    |
| /auth/api/oauth/token/refresh | POST | - |

Query Param:
```
    grant_type: refresh_token （固定字段）
    client_id:
```

Body Data (Forms):
```
    client_secret:
    refresh_token:
```

Response Data:
```json
{
    "access_token": "",
    "access_expired": 0, // 过期时间（s）
    "refresh_token": "",
    "refresh_expired": 0 // 过期时间（s）
}
```

##### 客户端注册与存储

| Path | Method | Header |
| ---  | ---    | ---    |
| /auth/api/oauth/store | POST | token |

该接口要求 OAuth access token 对应本地用户，且用户 `role_id = 2` 或角色 permissions 命中 OAuth client 管理权限。登记域名必须是 HTTPS origin，例如 `https://pass.muxixyz.com`；不允许 HTTP、localhost、IP、通配符、路径、query 或 fragment。CAS OAuth 的 `callback_url` 也必须使用 HTTPS，且 origin 必须与客户端登记域名完全一致。

Body Data:
```json
{
    "domain": "" // 域名
}
```

Response Data:
```json
{
    "client_id": "",
    "client_secret": ""
}
```

#### CAS OAuth Callback

当前服务在保留原有“用户名 / 密码 OAuth 授权”流程的同时，也支持基于 CAS 的授权码模式。

客户端需要自行拼接 CAS 登录地址，并将 `service` 参数指向：

```text
/auth/api/oauth/cas/callback?client_id=...&callback_url=...&token_exp=...
```

当 CAS 认证成功后，本认证服务会执行以下步骤：

1. 校验 CAS 返回的 ticket
2. 通过 `user_identities` 将 CAS 用户解析或自动创建为本地用户
3. 以本地 `users.id` 作为 OAuth subject 生成授权码
4. 重定向到 `callback_url?code=...`

## CAS 接入补充说明

为了避免继续把历史 README 的编码问题越改越乱，CAS 适配和本地联调说明已经单独整理到：

[docs/cas-oauth-debug.md](./docs/cas-oauth-debug.md)

文档里包含：

- CAS callback 的真实处理流程
- `cas.server_url` 与 `cas.callback_base_url` 的配置含义
- 本地启动 CAS 与 OAuth 服务的步骤
- 如何拼接 CAS 登录 URL
- 如何从授权码继续换取 `access_token`
- 常见联调报错的排查方法

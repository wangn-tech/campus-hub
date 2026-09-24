# HTTP API 设计规范

## 1. 文档目的

本文定义新 CampusHub 后端的 HTTP API 通用约定和接口清单。所有新接口统一使用 `/api/v1` 前缀，旧接口仅在迁移期通过兼容路由映射。

## 2. 通用约定

### 2.1 URL 前缀

```text
/api/v1
```

健康检查除外：

- `/health`；
- `/ready`。

### 2.2 请求与响应

- 请求体默认使用 `application/json`；
- 文件上传使用 `multipart/form-data`；
- 时间字段在数据库中使用 `DATETIME(3)`；
- API 响应中的时间可同时支持 RFC3339 和 Unix 时间戳，具体在 DTO 中固定；
- 所有接口使用 UTF-8 编码。

### 2.3 鉴权

需要登录的接口使用：

```text
Authorization: Bearer <access_token>
```

当前用户 ID 从 Access Token 中获取，不要求前端在请求体中重复传 `user_id`。

### 2.4 统一响应结构

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {},
  "trace_id": "trace-id"
}
```

失败响应：

```json
{
  "code": 100400,
  "message": "请求参数错误",
  "data": null,
  "trace_id": "trace-id"
}
```

### 2.5 分页约定

分页请求参数：

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `page` | int | 1 | 页码，从 1 开始 |
| `page_size` | int | 10 | 每页数量，最大 50 |

分页响应：

```json
{
  "items": [],
  "pagination": {
    "page": 1,
    "page_size": 10,
    "total": 100
  }
}
```

### 2.6 错误码分段

| 分段 | 含义 |
|---|---|
| `100400-100499` | 请求参数、鉴权、权限等客户端错误 |
| `100500-100599` | 服务端内部错误 |
| `101xxx` | 用户与认证域 |
| `102xxx` | 活动域 |
| `103xxx` | 报名与票据域 |
| `104xxx` | 聊天与通知域 |
| `105xxx` | 文件与搜索域 |

HTTP 状态码仍应正确返回，例如 400、401、403、404、429、500。

## 3. 健康检查

| Method | Path | 说明 |
|---|---|---|
| GET | `/health` | 进程存活检查 |
| GET | `/ready` | MySQL、Redis、Kafka、ES 依赖就绪检查 |

## 4. 认证接口

| Method | Path | 说明 | 请求体 | 鉴权 |
|---|---|---|---|---|
| POST | `/api/v1/auth/register` | 用户注册 | 邮箱、密码、昵称、验证码 | 否 |
| POST | `/api/v1/auth/login` | 用户登录 | 邮箱、密码 | 否 |
| POST | `/api/v1/auth/logout` | 退出登录 | 无 | 是 |
| POST | `/api/v1/auth/refresh` | 刷新 Access Token | Refresh Token | 刷新令牌 |
| POST | `/api/v1/auth/logoff` | 注销账号 | 验证码、密码确认 | 是 |
| POST | `/api/v1/auth/password/reset` | 忘记密码并重置 | 邮箱、验证码、新密码 | 否 |

旧路径映射：

- `/api/v1/login` → `/api/v1/auth/login`；
- `/api/v1/register` → `/api/v1/auth/register`；
- `/api/v1/logout` → `/api/v1/auth/logout`；
- `/api/v1/refresh_token` → `/api/v1/auth/refresh`；
- `/api/v1/logoff` → `/api/v1/auth/logoff`。

## 5. 用户接口

| Method | Path | 说明 | 鉴权 |
|---|---|---|---|
| GET | `/api/v1/users/me` | 获取当前用户资料 | 是 |
| PUT | `/api/v1/users/me` | 更新当前用户资料 | 是 |
| GET | `/api/v1/users/{id}/home` | 获取用户公开主页 | 否 |
| PUT | `/api/v1/users/me/password` | 修改密码 | 是 |
| GET | `/api/v1/users/{id}/status` | 获取用户在线状态 | 是 |
| GET | `/api/v1/users/me/groups` | 获取当前用户群聊列表 | 是 |
| GET | `/api/v1/users/me/activities/created` | 获取我创建的活动 | 是 |
| GET | `/api/v1/users/me/activities/registered` | 获取我报名的活动 | 是 |
| GET | `/api/v1/users/me/credit-logs` | 获取信用分变更记录 | 是 |

`/users/me/activities/registered` 支持参数：

- `status=upcoming`：待参加；
- `status=history`：已参加；
- `page`；
- `page_size`。

## 6. 兴趣与标签接口

| Method | Path | 说明 | 鉴权 |
|---|---|---|---|
| GET | `/api/v1/tags` | 获取标签列表 | 否/按场景 |
| GET | `/api/v1/tags?type=activity` | 获取活动标签 | 是 |
| GET | `/api/v1/tags?type=interest` | 获取兴趣标签 | 否 |
| PUT | `/api/v1/users/me/interests` | 更新当前用户兴趣标签 | 是 |

旧路径映射：

- `/api/v1/activity/tags` → `/api/v1/tags?type=activity`；
- `/api/v1/interests/tags` → `/api/v1/tags?type=interest`；
- `/api/v1/interests` → `/api/v1/users/me/interests`。

## 7. 分类接口

| Method | Path | 说明 | 鉴权 |
|---|---|---|---|
| GET | `/api/v1/categories` | 获取启用的活动分类 | 否 |

旧路径映射：

- `/api/v1/activity/categories` → `/api/v1/categories`。

## 8. 活动接口

| Method | Path | 说明 | 鉴权 |
|---|---|---|---|
| GET | `/api/v1/activities` | 获取活动列表 | 否 |
| GET | `/api/v1/activities/search` | 搜索活动 | 否 |
| GET | `/api/v1/activities/{id}` | 获取活动详情 | 否 |
| POST | `/api/v1/activities` | 创建活动 | 是 |
| PUT | `/api/v1/activities/{id}` | 更新活动 | 活动组织者/管理员 |
| POST | `/api/v1/activities/{id}/submit` | 提交审核 | 活动组织者 |
| POST | `/api/v1/activities/{id}/cancel` | 取消活动 | 活动组织者/管理员 |

活动列表支持：

- `category_id`；
- `status`；
- `page`；
- `page_size`；
- `sort`。

活动搜索支持：

- `keyword`；
- `category_id`；
- `tag_id`；
- `start_time`；
- `end_time`；
- `location`；
- `page`；
- `page_size`。

旧路径映射：

- `/api/v1/activity/lists` → `/api/v1/activities`；
- `/api/v1/activity/list` → `/api/v1/users/me/activities/registered`；
- `/api/v1/activity/search` → `/api/v1/activities/search`；
- `/api/v1/activity/{id}` → `/api/v1/activities/{id}`；
- `/api/v1/activity/` → `/api/v1/activities`。

## 9. 报名接口

| Method | Path | 说明 | 鉴权 |
|---|---|---|---|
| POST | `/api/v1/activities/{id}/registrations` | 报名活动 | 是 |
| GET | `/api/v1/activities/{id}/registrations` | 查看活动报名记录 | 活动组织者/管理员 |
| GET | `/api/v1/registrations/{id}` | 查看报名详情 | 报名者/组织者/管理员 |
| POST | `/api/v1/registrations/{id}/approve` | 审批通过 | 活动组织者/管理员 |
| POST | `/api/v1/registrations/{id}/reject` | 审批拒绝 | 活动组织者/管理员 |
| DELETE | `/api/v1/registrations/{id}` | 取消报名 | 报名者 |

报名响应按活动配置区分：

- 无需审批：返回 `approved` 状态、报名记录、票据信息和活动群信息；
- 需要审批：返回 `pending` 状态和报名记录，暂不返回票据或活动群；
- 审批通过后，由审批接口返回票据和活动群信息，并异步推送通知；
- 审批拒绝、取消或超时后，报名进入终态并释放预留名额。

旧路径映射：

- POST `/api/v1/activity/register` → POST `/api/v1/activities/{id}/registrations`；
- POST `/api/v1/activity/cancel` → DELETE `/api/v1/registrations/{id}`。

## 10. 票据接口

| Method | Path | 说明 | 鉴权 |
|---|---|---|---|
| GET | `/api/v1/tickets` | 获取当前用户票据列表 | 是 |
| GET | `/api/v1/tickets/{id}` | 获取票据详情 | 持票人/组织者/管理员 |

旧路径映射：

- `/api/v1/activity/tickets` → `/api/v1/tickets`；
- `/api/v1/activity/tickets/detail?ticketId={id}` → `/api/v1/tickets/{id}`。

## 11. 核销接口

| Method | Path | 说明 | 鉴权 |
|---|---|---|---|
| POST | `/api/v1/check-ins` | 核销票据 | 活动组织者/管理员 |
| GET | `/api/v1/check-ins` | 查看核销记录 | 活动组织者/管理员 |

核销请求：

- 票据码或票据 UUID；
- 活动 ID，可选；
- 经度，可选；
- 纬度，可选；
- 客户端幂等 ID。

旧路径映射：

- `/api/v1/activity/verify` → `/api/v1/check-ins`。

## 12. 学生认证接口

| Method | Path | 说明 | 鉴权 |
|---|---|---|---|
| GET | `/api/v1/student-verifications/current` | 获取当前认证进度 | 是 |
| POST | `/api/v1/student-verifications` | 提交认证申请 | 是 |
| POST | `/api/v1/student-verifications/{id}/confirm` | 确认 OCR 结果 | 申请人 |
| POST | `/api/v1/student-verifications/{id}/cancel` | 取消认证 | 申请人 |

旧路径映射：

- `/api/v1/verify/student/current` → `/api/v1/student-verifications/current`；
- `/api/v1/verify/student/apply` → `/api/v1/student-verifications`；
- `/api/v1/verify/student/confirm` → `/api/v1/student-verifications/{id}/confirm`；
- `/api/v1/verify/student/cancel` → `/api/v1/student-verifications/{id}/cancel`。

## 13. 验证码接口

| Method | Path | 说明 | 鉴权 |
|---|---|---|---|
| GET | `/api/v1/captcha/config` | 获取验证码配置 | 否 |
| GET | `/api/v1/captcha` | 获取验证码图片或题目 | 否 |
| POST | `/api/v1/captcha/verify` | 校验验证码 | 否 |
| GET | `/api/v1/email-codes` | 发送邮箱验证码 | 按场景判断 |

邮箱验证码场景：

- `register`；
- `forgot_password`；
- `logoff`。

旧路径映射：

- `/api/v1/qq_code/register` → `/api/v1/email-codes?scene=register`；
- `/api/v1/qq_code/forgot_password` → `/api/v1/email-codes?scene=forgot_password`；
- `/api/v1/qq_code/delete_user` → `/api/v1/email-codes?scene=logoff`。

## 14. 通知接口

| Method | Path | 说明 | 鉴权 |
|---|---|---|---|
| GET | `/api/v1/notifications` | 获取通知列表 | 是 |
| GET | `/api/v1/notifications/unread-count` | 获取未读数量 | 是 |
| POST | `/api/v1/notifications/read` | 标记通知已读 | 是 |
| POST | `/api/v1/notifications/read-all` | 全部标记已读 | 是 |

旧路径以 `/api/notifications` 为前缀，统一映射到 `/api/v1/notifications` 前缀。

## 15. 群聊与消息接口

| Method | Path | 说明 | 鉴权 |
|---|---|---|---|
| GET | `/api/v1/groups` | 获取当前用户群列表，可由用户接口替代 | 是 |
| GET | `/api/v1/groups/{id}` | 获取群信息 | 群成员 |
| GET | `/api/v1/groups/{id}/members` | 获取群成员 | 群成员 |
| GET | `/api/v1/groups/{id}/messages` | 获取群历史消息 | 群成员 |
| GET | `/api/v1/messages/offline` | 获取离线消息 | 是 |

旧路径映射：

- `/api/users/{user_id}/groups` → `/api/v1/users/me/groups`；
- `/api/groups/{group_id}` → `/api/v1/groups/{id}`；
- `/api/groups/{group_id}/members` → `/api/v1/groups/{id}/members`；
- `/api/messages` → `/api/v1/groups/{id}/messages`；
- `/api/messages/offline` → `/api/v1/messages/offline`；
- `/api/users/status` → `/api/v1/users/{id}/status`。

## 16. 文件接口

| Method | Path | 说明 | 鉴权 |
|---|---|---|---|
| POST | `/api/v1/files/images` | 上传图片 | 是 |
| GET | `/api/v1/files/{id}` | 获取文件元数据 | 按业务权限 |
| DELETE | `/api/v1/files/{id}` | 删除文件 | 上传者/管理员 |

旧路径映射：

- `/api/v1/images/upload` → `/api/v1/files/images`。

## 17. 幂等要求

以下接口应支持幂等：

- 报名；
- 票据核销；
- 文件上传确认；
- 通知已读；
- Kafka 消费触发的业务操作。

核销接口必须要求或生成唯一请求 ID，并通过唯一索引避免重复核销。

## 18. API 兼容策略

迁移期建议：

1. 新后端同时注册新路由和旧路由兼容映射；
2. 响应中可通过响应头提示旧接口废弃；
3. 前端完成切换并稳定后删除旧路由；
4. 兼容代码集中在路由适配层，不侵入新 Service；
5. 不再新增旧风格接口。

# AGENTS.md

CampusHub 是校园活动平台（活动发布、报名、票据核销、活动沟通）。本文件是仓库级指令，Codex 在开始工作前会读取它；更靠近工作目录的 `AGENTS.override.md` 或 `AGENTS.md` 会覆盖本文件的对应内容。

## 仓库结构

- `backend/` — Go 1.24 模块化单体（Gin + GORM + MySQL + Redis + Kafka + Elasticsearch + RustFS），入口 `cmd/server`、`cmd/migrate`。
- `frontend/` — uni-app + Vue 3，用 npm。
- `docs/system_design/` — 设计文档，是接口、配置、状态与数据的**契约来源**。
- `backend/deploy/docker/` — `docker-compose.yml`（本地中间件）与 `Dockerfile`。

## 常用命令（均在 `backend/` 下执行）

| 目的 | 命令 |
|---|---|
| 启动本地中间件 | `make dev-up` |
| 停止中间件 | `make dev-down` |
| 应用 / 回滚迁移 | `make migrate-up` / `make migrate-down` |
| 启动服务 | `make run` |
| 单元测试 | `make test` |
| 格式化 + vet | `make lint`（`fmt` 会直接改写文件） |
| 端到端集成测试 | `make integration`（需要 Docker，会拉起整套 Compose） |
| 校验 Compose | `make compose-config` |

提交前至少跑一遍与 CI 相同的门禁：

```sh
cd backend
test -z "$(gofmt -l .)" && go vet ./... && go test -race ./... && go build ./cmd/server ./cmd/migrate
```

前端改动：`cd frontend && npm ci && npm run type-check`（CI 还会跑 `npm run build:h5:ssr` 与 `npm run build:mp-weixin`）。

## 分层与依赖方向（不要打破）

依赖方向是 `handler → service → repository → model`，`router` 只负责装配 `handler` 与 `middleware`。

- `internal/middleware` **不得** import `internal/service`。需要业务能力时在本包定义接口（如 `Authenticator`、`AdminChecker`），由 `router` 注入实现。
- `internal/service` 不得 import `internal/handler`、`internal/middleware`、`internal/router`。
- JWT 的签发与校验在 `internal/token`；外部依赖适配器放 `internal/platform/*`。
- 改完依赖后可自查：`go list -f '{{join .Imports "\n"}}' ./internal/<pkg>`。

## API 约定

- 统一响应体 `httpx.Response{code,message,data,trace_id}`，成功时 `code=0`；出错走 `httpx.Error`。
- 业务码分段：`1004xx` 客户端错误、`1005xx` 服务端错误、`101xxx` 认证、`102xxx` 活动、`103xxx` 报名票据、`104xxx` 聊天通知、`105xxx` 文件搜索。
- 分页统一用 `httpx.Page[T]`：`items` + `pagination{page,page_size,total}`；`page` 默认 1，`page_size` 默认 10、上限 50。
- 时间字段一律 Unix 毫秒（`internal/platform/timestamp`），数据库的 `DATETIME(3)` 不外露。
- 对外标识一律 UUID 字符串，自增主键只在内部使用。
- JSON 字段用 snake_case；资源路径用复数（`/api/v1/activities`）；不要再新增旧风格 `/api/v1/activity/*`。
- 枚举数值必须与 `docs/system_design/09-数据库设计.md` §16 一致；改枚举等于改协议。

## 数据与迁移

- 迁移文件位于 `backend/migrations/mysql/NNNNNN_name.up.sql` / `.down.sql`，由 golang-migrate 执行（DSN 已开启 `multiStatements`）。
- 已合入的迁移**不要修改**，只追加新迁移；每个 up 都要有可用的 down。
- 状态机变更必须在同一事务内写 `*_status_logs`（参考 `activity_status_logs`）。
- GORM 陷阱：`[]uint8` 就是 `[]byte`，传给 `IN ?` 会被当成单个二进制值 → 用 `[]int`。

## 文件存储（RustFS）

- 只存文件 ID（如 `cover_file_id`）并写 `file_references(resource_type, resource_id, purpose)`；**不要**把预签名 URL 落库（15 分钟即失效）。
- 读取时用 `FileService.AccessURL` 现签 URL。

## 认证与权限

- 鉴权中间件：`Auth`（强制）、`OptionalAuth`（可选，供公开详情页识别本人或管理员）。从上下文取用户用 `middleware.UserUUID(c)`，不要直接 `MustGet`。
- 管理员判定基于 `user_roles → roles.code = 'admin'`（`middleware.RequireAdmin`）。注册流程不自动赋权，管理员需用 SQL 授予：

```sql
INSERT IGNORE INTO user_roles (user_id, role_id, created_at)
SELECT u.id, r.id, UTC_TIMESTAMP(3)
FROM users u JOIN roles r ON r.code = 'admin'
WHERE u.email = 'admin@example.com';
```

## 配置

- Viper：环境变量前缀 `CAMPUSHUB_`，用 `CAMPUSHUB_CONFIG_FILE` 指定配置文件（默认 `configs/config.dev.yaml`）。
- 新增配置必须给默认值并纳入 `config.Validate()`。
- 只有 `.env.example` 可以提交，真实密钥一律通过环境变量注入。

## 文档同步

改了以下内容就要同步对应文档，否则视为未完成：

- 接口路径 / 字段 / 错误码 → `02-前端接口清单.md`、`07-HTTP API 设计规范.md`、`18-前端更新.md`，以及 `backend/api/openapi.yaml`（CI 目前只校验该文件非空）。
- 表结构与状态枚举 → `09-数据库设计.md`。
- 配置项 → `13-配置设计.md`。
- 依赖组件与 readiness 契约 → `04-架构总览.md`、`14-Docker 部署设计.md`、`16-可观测性设计.md`。

## 测试

- 纯逻辑（参数校验、状态允许性、分页、字段映射）写表驱动单测；不要为了可测性把数据库逻辑硬塞进单测。
- 跨层行为（HTTP → service → MySQL/Redis/RustFS）由 `backend/scripts/ci-integration.sh` 覆盖；新增业务闭环时扩展该脚本。
- 该脚本会对对象存储与 Redis 做故障恢复断言，改动这些路径时必须让它继续通过。

## Git 与 CI

- 提交信息用 Conventional Commits：`feat|fix|refactor|docs|test|ci|chore(scope): ...`。
- 改动通过 PR 合入 `main`，合并前 CI 必须全绿。
- CI 定义在 `.github/workflows/backend-ci.yml`，只在 `backend/**` 或该工作流自身变更时触发；`quality` 与 `integration` 并行，热缓存下约 1 分 40 秒。
- 不要为通过 CI 而跳过或注释测试。CI 会校验 `go mod tidy` 后 `go.mod`/`go.sum` 无差异，改依赖后先在本地跑一次。

## Code Review Rules

- **数据库 `IN` 查询**：把 `[]uint8` / `[]byte` 传给 `IN ?` 会生成非法 SQL。应改为 `[]int`，并补一个覆盖该列表的测试。
- **持久化枚举**：改动任何状态数值（activity、registration、ticket、verification）必须同时更新 `09-数据库设计.md` §16 与迁移，否则标记为需修改。
- **状态变更未记日志**：只改 `status` 而未在同一事务写 `*_status_logs`，恢复与审计路径会失效，应拦下。
- **预签名 URL 落库**：把 presigned/signed URL 写进数据库或返回长期 URL 应拦下，改为存文件 ID、读取时现签。
- **分层破环**：`middleware` import `service`，或 `service` import `handler`/`middleware`/`router`，应改为接口注入。
- **readiness 契约变更**：增删 `/ready` 的依赖项时必须同步 `16-可观测性设计.md` 与集成测试。
- **密钥入库**：任何真实密钥、令牌或默认生产口令进入受版本控制的文件都要拦下。

## 给 agent 的提醒

- 不要提交 `.env`、真实密钥或本地生成物；`.env.example` 只是模板。
- 本地起中间件前确认端口空闲：MySQL `13306`、Redis `16379`、Kafka `19092`、Elasticsearch `19200`、RustFS `19000`/`19001`、Mailpit `1025`/`18025`。
- 行为拿不准时以 `docs/system_design/` 为准；文档与代码冲突时先反馈，不要静默选一边。

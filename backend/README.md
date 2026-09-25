# CampusHub Backend

后端采用 Go 模块化单体。宿主机运行 Go 服务，Docker Compose 只负责本地中间件；业务代码通过迁移脚本管理数据库结构，不使用 GORM AutoMigrate。

## 快速开始

```sh
cp .env.example .env
make dev-up
make migrate-up
make run
```

默认宿主机端口：MySQL `13306`、Redis `16379`、Kafka `19092`、Elasticsearch `19200`、MinIO `19000`（控制台 `19001`）及 Mailpit `18025`。服务监听 `8080`，Compose 中可选的后端容器映射为 `18080`。

常用命令：

- `make test`：运行全部 Go 测试。
- `make lint`：执行格式化和 `go vet`。
- `make migrate-up` / `make migrate-down`：执行或回滚一条迁移。
- `make compose-config`：校验 Compose 文件和环境变量模板。
- `make integration`：启动本地中间件并验证迁移、健康检查、认证和 Redis 故障恢复；会停止本次启动的 Compose 服务，但保留本地卷。

`GET /health` 只表示进程存活。`GET /ready` 会并发检查 MySQL、Redis、Kafka 与 Elasticsearch：全部可用时返回 `200` 和 `ready`，任一不可用时返回 `503` 和各依赖的 `up/down` 状态。服务启动时不因依赖暂时离线退出；负载均衡应仅向 ready 实例转发业务流量。

文件存储使用私有 MinIO Bucket，首次上传时自动创建配置的 Bucket。Compose 固定使用从 MinIO 源码构建的可复现镜像 digest，避免上游已下线镜像影响 CI。上传接口仅接受 JPEG、PNG、WebP 和 GIF（最大 5 MiB），并只返回 15 分钟有效的预签名 URL；MinIO 暂时不可用时仅文件接口返回 `503`，不影响现有 readiness 契约。本地验证码邮件投递到 Mailpit；生产必须以 `CAMPUSHUB_MAIL_*`、`CAMPUSHUB_STORAGE_*` 和 `CAMPUSHUB_SECURITY_PII_*` 注入真实配置。

API、WebSocket 和 Kafka 的时间字段使用 Unix 毫秒整数；数据库内部使用 `DATETIME(3)`，转换统一由 `internal/platform/timestamp` 完成。真实密钥只通过环境变量注入，不提交 `.env`。

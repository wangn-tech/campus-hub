# CampusHub Backend

后端采用 Go 模块化单体。宿主机运行 Go 服务，Docker Compose 只负责本地中间件；业务代码通过迁移脚本管理数据库结构，不使用 GORM AutoMigrate。

## 快速开始

```sh
cp .env.example .env
make dev-up
make migrate-up
make run
```

默认宿主机端口：MySQL `13306`、Redis `16379`、Kafka `19092`、Elasticsearch `19200`。服务监听 `8080`，Compose 中可选的后端容器映射为 `18080`。

常用命令：

- `make test`：运行全部 Go 测试。
- `make lint`：执行格式化和 `go vet`。
- `make migrate-up` / `make migrate-down`：执行或回滚一条迁移。
- `make compose-config`：校验 Compose 文件和环境变量模板。

API、WebSocket 和 Kafka 的时间字段使用 Unix 毫秒整数；数据库内部使用 `DATETIME(3)`，转换统一由 `internal/platform/timestamp` 完成。真实密钥只通过环境变量注入，不提交 `.env`。

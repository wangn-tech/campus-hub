# Docker 部署设计

## 1. 文档目的

本文规划 CampusHub 的 Docker 镜像、Docker Compose 服务、网络、数据卷、健康检查、环境变量、本地开发和生产部署注意事项。

## 2. 容器化目标

- 一条命令启动本地开发依赖；
- 保持开发、测试、生产环境一致；
- 后端镜像支持多阶段构建；
- 数据持久化；
- 可选组件通过 Profile 控制；
- 容器具备健康检查和优雅停机能力。

## 3. Compose 服务规划

### 3.1 核心服务

| 服务 | 镜像/构建 | 说明 |
|---|---|---|
| `app` | 本地 Dockerfile 构建 | CampusHub Go 后端 |
| `mysql` | MySQL 8.x | 业务主库 |
| `redis` | Redis 7.x | 缓存、锁、在线状态 |
| `kafka` | Kafka KRaft 模式 | 消息队列 |
| `elasticsearch` | Elasticsearch 9.x | 活动搜索 |

### 3.2 可选服务

| 服务 | Compose Profile | 说明 |
|---|---|---|
| `kibana` | `search` | ES 查询和索引调试 |
| `kafka-ui` | `mq` | Kafka Topic、消息和消费者组调试 |
| `minio` | `storage` | 本地 S3 对象存储 |
| `prometheus` | `monitoring` | 指标采集 |
| `grafana` | `monitoring` | 指标展示 |

## 4. 推荐 Compose 结构

```text
deploy/docker/
├── docker-compose.yml
├── docker-compose.dev.yml
├── docker-compose.prod.yml
└── env/
    └── app.env.example
```

基础服务放在 `docker-compose.yml`，环境差异通过 override 文件或环境变量控制。

## 5. 网络规划

建议创建独立网络，例如：

```text
campushub-net
```

服务之间使用服务名访问：

- `mysql:3306`；
- `redis:6379`；
- `kafka:9092`；
- `elasticsearch:9200`；
- `minio:9000`。

后端容器只暴露必要端口，中间件端口可只在开发环境映射到宿主机。

## 6. 数据卷规划

| 卷 | 挂载服务 | 说明 |
|---|---|---|
| `mysql-data` | mysql | MySQL 数据 |
| `redis-data` | redis | Redis 持久化数据 |
| `kafka-data` | kafka | Kafka 数据 |
| `elasticsearch-data` | elasticsearch | ES 数据 |
| `minio-data` | minio | 文件数据 |
| `prometheus-data` | prometheus | 监控数据 |
| `grafana-data` | grafana | 仪表盘数据 |

生产环境应使用可靠存储，并根据数据库和搜索集群策略配置备份。

## 7. 后端镜像设计

### 7.1 多阶段构建

建议阶段：

1. Go build 阶段：编译后端二进制；
2. Runtime 阶段：使用精简基础镜像运行；
3. 非 root 用户运行；
4. 复制必要配置和迁移文件；
5. 暴露 HTTP 端口；
6. 使用健康检查；
7. 使用 JSON 日志输出到 stdout。

### 7.2 构建参数与环境变量

构建时可传入：

- Go 版本；
- 应用版本；
- Commit SHA；
- 构建时间。

运行时通过环境变量注入：

- 环境；
- MySQL、Redis、Kafka、ES 地址；
- JWT 密钥；
- 对象存储配置；
- 功能开关。

## 8. MySQL 容器

配置要点：

- 设置数据库名、用户名、密码；
- 使用 `utf8mb4`；
- 使用独立数据卷；
- 配置健康检查；
- 应用等待 MySQL 就绪后再启动；
- 初始化脚本只用于本地开发，不替代正式迁移。

生产环境建议使用托管数据库或独立 MySQL 高可用方案。

## 9. Redis 容器

配置要点：

- 设置密码，生产环境必须启用；
- 配置持久化策略；
- 设置最大内存和淘汰策略；
- 健康检查；
- 应用连接使用服务名。

## 10. Kafka 容器

配置要点：

- 使用 KRaft 模式，避免额外 ZooKeeper；
- 配置单节点或集群 ID；
- 配置内部和外部监听地址；
- 持久化数据卷；
- 健康检查；
- Kafka UI 通过 Profile 启动。

开发环境单节点即可，生产环境应配置多 Broker、副本和认证授权。

## 11. Elasticsearch 容器

配置要点：

- 单节点模式用于开发；
- 配置 JVM/内存限制；
- 启用安全配置时同步配置用户名和密码；
- 持久化 ES 数据卷；
- Kibana 依赖 ES 健康状态；
- 禁用或调整 swap。

生产环境应使用独立 ES 集群或托管服务，并配置副本、快照和访问控制。

## 12. MinIO 容器

配置要点：

- 设置 Access Key 和 Secret Key；
- 创建或自动初始化 Bucket；
- 暴露 API 和控制台端口；
- 配置数据卷；
- 后端使用 S3 兼容协议访问。

## 13. 健康检查与依赖顺序

### 13.1 健康检查

| 服务 | 检查内容 |
|---|---|
| app | `/health` |
| mysql | 数据库连接或管理命令 |
| redis | ping |
| kafka | Broker 健康命令或端口检查 |
| elasticsearch | 集群健康接口 |
| minio | 存活探针 |
| kibana | HTTP 状态接口 |

### 13.2 依赖关系

应用依赖：

- MySQL；
- Redis；
- Kafka；
- Elasticsearch；
- MinIO，如启用。

Compose 可配置 `depends_on` 和条件健康检查，但应用自身也应具备连接重试能力。

## 14. 本地开发启动流程

推荐流程：

1. 启动核心中间件；
2. 执行数据库迁移；
3. 初始化必要字典数据；
4. 启动后端服务；
5. 启动前端或使用已有前端子模块；
6. 按需启动 Kibana、Kafka UI、MinIO 控制台。

## 15. 生产部署注意事项

生产环境不应直接使用开发 Compose 文件，应关注：

- 镜像使用固定版本，不使用 `latest`；
- 数据库密码和密钥通过环境变量或密钥系统注入；
- MySQL、Kafka、Elasticsearch 高可用；
- 数据备份和恢复演练；
- 容器资源限制；
- 日志采集；
- 负载均衡和 HTTPS；
- WebSocket 反向代理超时和连接升级；
- 滚动发布和优雅停机；
- 监控和告警。

## 16. WebSocket 反向代理要求

网关或 Nginx 需要支持：

- Upgrade；
- Connection upgrade；
- 较长读超时；
- 客户端真实 IP；
- 多实例负载均衡；
- Sticky Session 不是必须，因为连接信息保存在 Redis，消息通过 Kafka 广播。

## 17. 环境变量示例分类

应提供 `.env.example`，包含：

- MySQL 初始化配置；
- Redis 密码；
- Kafka 监听配置；
- ES 内存和安全配置；
- MinIO 凭证；
- 后端配置覆盖；
- Compose Profile。

真实 `.env` 文件不提交。

## 18. 交付前检查

1. 所有核心服务可以通过 Compose 正常启动。
2. 后端容器能连接全部依赖。
3. 数据卷重启后数据仍保留。
4. 健康检查状态正确。
5. 停止后端容器时正在处理的请求能优雅完成。
6. 可选 Profile 不影响核心服务启动。
7.. 文档中的服务名、端口和环境变量与实际 Compose 一致。

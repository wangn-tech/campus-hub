# CampusHub 系统设计文档

本目录存放 CampusHub 新后端重构的系统设计文档。文档只描述开发前的分析、设计、约定和迁移计划，不包含业务代码实现。

## 技术基线

新后端技术栈：

- Go 1.24+；
- Gin；
- GORM；
- MySQL 8；
- Redis 7；
- Kafka，KRaft 模式；
- Elasticsearch 9；
- WebSocket；
- Ristretto v2 本地缓存；
- Viper；
- Docker / Docker Compose。

核心原则：

- MySQL 是唯一业务事实来源；
- Elasticsearch 只承担活动搜索和聚合读模型；
- Kafka 承担异步事件和最终一致性；
- Redis 承担缓存、Token、验证码、锁和在线状态；
- WebSocket 只负责实时连接和消息投递；
- 一期 Elasticsearch 只索引活动数据。

## 文档导航

| 编号 | 文档 | 内容 |
|---|---|---|
| 01 | [旧系统分析](./01-legacy-system-analysis.md) | 旧 go-zero 后端的模块、数据、协作流程、问题和可复用规则 |
| 02 | [前端接口清单](./02-frontend-api-inventory.md) | 前端现有 HTTP 接口、WebSocket 事件、旧路径和新系统处理建议 |
| 03 | [业务领域设计](./03-business-domain.md) | 业务域、角色、实体关系、权限矩阵和状态机 |
| 04 | [架构总览](./04-architecture-overview.md) | 系统架构、组件关系、关键链路、多实例和降级策略 |
| 05 | [技术栈选型](./05-tech-stack.md) | Go 依赖、中间件客户端、第三方库和替代方案 |
| 06 | [项目结构设计](./06-project-structure.md) | 推荐目录结构、包职责和依赖方向 |
| 07 | [HTTP API 设计规范](./07-http-api-spec.md) | API 通用约定、错误码、分页、接口清单和旧路径映射 |
| 08 | [WebSocket 协议设计](./08-websocket-protocol.md) | 连接认证、消息信封、心跳、ACK、事件类型和多实例路由 |
| 09 | [数据库设计](./09-database-design.md) | MySQL 表结构、字段、索引、约束、ER 图和枚举 |
| 10 | [Redis 与本地缓存设计](./10-redis-cache-design.md) | Redis Key、TTL、Ristretto、缓存一致性和失效策略 |
| 11 | [Elasticsearch 设计](./11-elasticsearch-design.md) | 活动索引、Mapping、查询、Kafka 同步和全量重建 |
| 12 | [Kafka 设计](./12-kafka-design.md) | Topic、事件结构、生产者、消费者、重试、死信和幂等 |
| 13 | [配置设计](./13-configuration.md) | Viper 加载方式、环境变量、配置项和敏感信息管理 |
| 14 | [Docker 部署设计](./14-docker-deployment.md) | 镜像、Compose 服务、网络、数据卷、健康检查和部署要求 |
| 15 | [安全设计](./15-security-design.md) | JWT、RBAC、密码、限流、文件安全和审计 |
| 16 | [可观测性设计](./16-observability.md) | 日志、指标、健康检查、链路追踪、告警和优雅启停 |
| 17 | [重构与迁移计划](./17-refactor-migration-plan.md) | 阶段计划、数据迁移、测试策略、风险、回滚和验收标准 |
| 18 | [前端更新](./18-前端更新.md) | 新后端完成后前端需要进行的接口、字段、状态、页面和实时通信改造 |

## 推荐阅读顺序

### 产品与业务视角

1. 业务领域设计；
2. 前端接口清单；
3. 前端更新；
4. HTTP API 设计规范；
5. WebSocket 协议设计。

### 后端开发视角

1. 架构总览；
2. 技术栈选型；
3. 项目结构设计；
4. 数据库设计；
5. Redis 与本地缓存设计；
6. Kafka 设计；
7. Elasticsearch 设计；
8. 配置设计。

### 运维与上线视角

1. Docker 部署设计；
2. 安全设计；
3. 可观测性设计；
4. 重构与迁移计划。

## 文档状态

| 状态 | 含义 |
|---|---|
| Draft | 初稿，待评审 |
| Review | 评审中 |
| Approved | 已确认，可作为开发依据 |
| Deprecated | 已废弃 |

当前文档状态：**Draft**。

## 维护规则

1. 架构、接口、数据库或中间件方案变化时，必须同步更新对应文档。
2. 文档中的路径、Topic、Key、枚举应保持唯一来源，避免多处定义冲突。
3. 图表优先使用 Mermaid。
4. 临时兼容策略必须标注删除条件或删除时间。
5. 文档不记录真实密码、密钥、Token 或生产连接串。
6. 评审通过后，文档状态更新为 Approved。

# CampusHub 校园活动平台

## 项目名称

CampusHub 校园活动与票据核销平台

## 项目简介

面向高校校园活动的全栈平台，支持活动创建与审核、公开检索、报名及审批、电子票据发放与核销、活动群聊、通知推送和学生认证。系统通过 HTTP 接口提供离线可追溯的数据查询，通过 WebSocket 提供实时提示，兼顾多实例部署下的一致性与可用性。

## 技术栈

- 后端：Go、Gin、GORM、MySQL、Redis、Kafka、Elasticsearch、RustFS。
- 前端：uni-app、Vue 3、TypeScript、Pinia，覆盖 H5 SSR 与微信小程序。
- 工程化：Docker Compose、GitHub Actions、OpenAPI、golang-migrate、go test -race、npm type-check。

## 项目亮点

- 设计多级缓存链路：Ristretto 本地缓存 → Redis → MySQL；缓存空值与短 TTL 防穿透，`singleflight` 合并热点 Key 回源防击穿，TTL 随机抖动、多级缓存与降级回源降低雪崩风险；写操作以事务提交后失效/双删和版本校验保证最终一致性。
- 通过数据库唯一键、乐观锁、状态机和 `client_request_id` 实现幂等：报名限制同一用户的有效报名，票据核销抵御重复请求；聊天使用 `client_message_id`，通知以 `(event_id, recipient)` 派生稳定 UUID，Kafka 重投不产生重复业务记录。
- 采用事务 Outbox + Kafka Relay 保障消息不丢：业务数据和 outbox 事件同一 MySQL 事务提交；Relay 以 `FOR UPDATE SKIP LOCKED` 认领、失败指数退避重试，发送成功后才标记已投递，进程故障后可重放未确认事件。
- 按至少一次语义处理 Kafka 重复：事件含唯一 `event_id`，消费者依赖业务唯一键、幂等写入和外部版本控制；活动索引使用 ES external version，避免旧事件覆盖新文档。
- 实现多实例 WebSocket：Redis 维护在线状态，Kafka 以实例级消费者组广播消息；连接认证、心跳、ACK、成员权限校验和消息落库后实时投递齐全。通知、认证和报名状态仅作为推送提示，断线或投递失败后以 HTTP 离线接口补偿。
- 基于 Elasticsearch 构建活动搜索读模型，支持关键词、分类、标签、公开状态、时间、地点、地理距离和多维排序；ES 不可用或索引未就绪时回退 MySQL，并通过 `X-Search-Mode` 暴露降级状态。索引重建采用新物理索引校验、alias 原子切换和补偿扫描，可回滚且避免切换窗口遗漏。

## 性能与压测指标

在单 Go 服务实例与本机 Docker 依赖（48 条活动数据、20 并发、30 秒）环境下完成核心读链路基线压测，所有请求均为 2xx、无传输错误；以下延迟单位均为毫秒（ms）：

- 活动列表：P50/P95/P99 为 24 / 31 / 35 ms（803.8 RPS）。
- Elasticsearch 活动搜索：P50/P95/P99 为 16 / 26 / 35 ms（1,137.6 RPS）。
- Elasticsearch 停止后的 MySQL 降级搜索：P50/P95/P99 为 11 / 16 / 20 ms（1,657.2 RPS）；验证 `X-Search-Mode` 能从 `mysql-fallback` 恢复至 `elasticsearch`。

上述数据是开发机基线，不作为生产容量承诺；后续将补充报名/核销写链路、Kafka 消费延迟和双实例 WebSocket 的隔离环境压测结果。完整命令、环境与结果见 `docs/test/results/2026-09-25-local-docker.md`。

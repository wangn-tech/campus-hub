# 2026-09-25 本地 Docker 核心读链路基线

## 环境

- Git SHA：`a14fc10`（压测器修复后的工作树，结果提交前的本地运行）。
- 运行方式：单个 Go 服务实例监听 `127.0.0.1:18080`；MySQL、Redis、Kafka、Elasticsearch、RustFS 通过 Docker Compose 启动。
- 主机资源：32 vCPU、15 GiB 内存；压测器与服务运行在同一主机，因此结果仅作开发环境基线，不能外推为生产容量。
- 数据量：48 条未删除活动；活动列表页大小为 10。无生产 Token、真实用户数据或写入压测流量。
- 压测器：`go run ./scripts/loadtest`；每个场景 20 并发、30 秒、默认仅接受 2xx。

## 结果

| 场景 | 模式 | 请求数 | RPS | P50 | P95 | P99 | HTTP/传输错误 |
|---|---|---:|---:|---:|---:|---:|---:|
| `activity-list` | MySQL 公开列表 | 24,114 | 803.8 | 24ms | 31ms | 35ms | 0 / 0 |
| `activity-search` | Elasticsearch | 34,129 | 1,137.6 | 16ms | 26ms | 35ms | 0 / 0 |
| `activity-search` | MySQL fallback | 49,716 | 1,657.2 | 11ms | 16ms | 20ms | 0 / 0 |

执行命令：

```sh
CAMPUSHUB_LOAD_BASE_URL=http://127.0.0.1:18080 \
  go run ./scripts/loadtest -scenario activity-list -concurrency 20 -duration 30s
CAMPUSHUB_LOAD_BASE_URL=http://127.0.0.1:18080 \
  go run ./scripts/loadtest -scenario activity-search -concurrency 20 -duration 30s
```

降级场景中先停止本地 Elasticsearch，确认 `X-Search-Mode: mysql-fallback` 后运行同一搜索命令；恢复 ES 后再次确认 `X-Search-Mode: elasticsearch`。

## 结论与限制

- 三个读场景均返回 2xx，未观察到传输错误；ES 停止后搜索正确进入 MySQL 降级，恢复后回到 ES。
- MySQL 降级快于 ES 是本机 48 条小数据集、同机 Docker 环境的现象，不能解读为生产查询策略结论。
- 本次没有执行报名、核销、Kafka/WebSocket 的并发写压：它们需要一次性测试账号、独立活动和双实例采集，避免污染共享数据或把 409 幂等冲突错误计入吞吐失败。后续应按《核心链路压测方案》完成这三类链路，并记录 outbox pending/failed、Kafka lag、消息端到端延迟和容量一致性核对结果。

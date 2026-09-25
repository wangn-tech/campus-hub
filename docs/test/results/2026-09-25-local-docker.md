# 2026-09-25 本地 Docker 核心读链路基线

## 环境

- 基线分支：`docs/load-testing`，以 `2021b14` 为起点；本次执行还包含 alias 切换与补偿扫描的兼容性修复。
- 运行方式：单个 Go 服务实例监听 `127.0.0.1:18080`；MySQL、Redis、Kafka、Elasticsearch、RustFS 通过 Docker Compose 启动。
- 主机资源：32 vCPU、15 GiB 内存；压测器与服务运行在同一主机，因此结果仅作开发环境基线，不能外推为生产容量。
- 数据量：48 条未删除活动；活动列表页大小为 10。无生产 Token、真实用户数据或写入压测流量。
- 压测器：`go run ./scripts/loadtest`，默认仅接受 2xx。列表阶梯为 30 秒；搜索与故障回退为 20 秒。

## 结果

| 场景 | 模式 | 并发 | 时长 | 请求数 | QPS | P50 | P95 | P99 | HTTP/传输错误 |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `activity-list` | 公开列表 | 20 | 30s | 24,114 | 803.8 | 24ms | 31ms | 35ms | 0 / 0 |
| `activity-list` | 公开列表 | 50 | 30s | 28,124 | 937.5 | 51ms | 72ms | 83ms | 0 / 0 |
| `activity-list` | 公开列表 | 100 | 30s | 27,585 | 919.5 | 105ms | 158ms | 188ms | 0 / 0 |
| `activity-list` | 公开列表 | 200 | 30s | 27,752 | 925.1 | 203ms | 362ms | 457ms | 0 / 0 |
| `activity-search` | Elasticsearch（48 条命中） | 20 | 20s | 10,410 | 520.5 | 34ms | 55ms | 77ms | 0 / 0 |
| `activity-search` | Elasticsearch（48 条命中） | 50 | 20s | 13,896 | 694.8 | 70ms | 97ms | 113ms | 0 / 0 |
| `activity-search` | MySQL fallback（48 条基础结果） | 20 | 20s | 13,299 | 665.0 | 29ms | 37ms | 42ms | 0 / 0 |

执行命令：

```sh
CAMPUSHUB_LOAD_BASE_URL=http://127.0.0.1:18080 \
  go run ./scripts/loadtest -scenario activity-list -concurrency 20 -duration 30s
CAMPUSHUB_LOAD_BASE_URL=http://127.0.0.1:18080 \
  go run ./scripts/loadtest -scenario activity-search -concurrency 20 -duration 20s
```

搜索压测前确认该请求的 `X-Search-Mode: elasticsearch`，并完成一次 48 条活动的 reindex。降级场景中停止本地 Elasticsearch，确认 `X-Search-Mode: mysql-fallback` 后运行同一搜索命令；恢复 ES、等待索引主分片 `STARTED` 后再次确认 `X-Search-Mode: elasticsearch`。本次两次模式确认均完成。

## 结论与限制

- 所有表中场景均返回 2xx，未观察到传输错误；ES 停止后搜索进入 MySQL 降级，分片恢复后回到 ES。
- 列表在 50 并发取得该机最高吞吐 937.5 QPS；升至 100/200 并发时吞吐平台化而 P95 上升至 158/362ms。因此本机开发基线的低延迟档为 20 并发，50 并发是吞吐观察点，不是生产容量承诺。
- ES 搜索需实际命中索引才能计为 ES 模式。旧的带 `keyword=campus` 样例在这批种子数据中无命中，会按“空结果回退”契约转为 MySQL，已不再作为 ES 指标。MySQL 降级快于 ES 是 48 条小数据集、同机 Docker 环境的现象，不能解读为生产查询策略结论。
- 本次还发现并修复重建链路的两个兼容性问题：alias remove 使用 ES 支持的 `must_exist: false`，补偿扫描使用 `external_gte` 接受相同版本的幂等重放。临时 `activities_loadtest` alias 完成 48 文档重建、校验、切换和补偿；旧索引保留。
- 本次没有执行报名、核销、Kafka/WebSocket 的并发写压：它们需要一次性测试账号、独立活动和双实例采集，避免污染共享数据或把 409 幂等冲突错误计入吞吐失败。后续应按《核心链路压测方案》完成这三类链路，并记录 outbox pending/failed、Kafka lag、消息端到端延迟和容量一致性核对结果。

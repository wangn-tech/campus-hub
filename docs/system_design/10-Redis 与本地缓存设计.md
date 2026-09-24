# Redis 与本地缓存设计

## 1. 文档目的

本文定义 CampusHub 中 Redis、Ristretto 本地缓存的使用场景、Key 规范、数据结构、TTL、失效策略和一致性方案。

## 2. 缓存分层

系统查询路径：

```text
本地缓存 Ristretto → Redis → MySQL
```

写入路径：

```text
Service → MySQL 事务 → Outbox / 缓存失效事件 → Redis / 本地缓存失效
```

缓存职责：

- Ristretto：进程内极热点、低频变更数据；
- Redis：跨实例共享缓存、Token、验证码、锁、在线状态；
- MySQL：最终事实来源。

## 3. Redis Key 命名规范

统一格式：

```text
campushub:{domain}:{resource}:{identifier}
```

环境隔离可使用数据库编号或前缀：

```text
campushub:dev:{domain}:{resource}:{identifier}
campushub:prod:{domain}:{resource}:{identifier}
```

Key 必须集中定义，不允许在业务代码中散落字符串。

## 4. Redis Key 清单

### 4.1 认证与 Token

| Key | 类型 | TTL | 说明 |
|---|---|---|---|
| `campushub:auth:refresh:{token_id}` | Hash/String | Refresh Token 有效期 | 保存用户 ID、设备、Token 摘要 |
| `campushub:auth:blacklist:{jti}` | String | Access Token 剩余有效期 | 退出登录后的 Token 黑名单 |
| `campushub:auth:login-fail:{email}` | String/Counter | 15 分钟 | 登录失败次数 |
| `campushub:auth:rate-limit:{user_id}` | String/Counter | 动态窗口 | 登录或敏感操作限流 |

### 4.2 验证码

| Key | 类型 | TTL | 说明 |
|---|---|---|---|
| `campushub:captcha:{captcha_id}` | String/Hash | 5 分钟 | 图形验证码答案和配置 |
| `campushub:email-code:{scene}:{email}` | String | 5-10 分钟 | 邮箱验证码 |
| `campushub:email-code:limit:{scene}:{email}` | Counter | 60 秒/1 小时 | 验证码发送频率限制 |

场景包括：

- `register`；
- `forgot_password`；
- `logoff`。

### 4.3 活动与分类缓存

| Key | 类型 | TTL | 说明 |
|---|---|---|---|
| `campushub:category:list` | String(JSON) | 30 分钟 | 启用分类列表 |
| `campushub:tag:list:{type}` | String(JSON) | 10-30 分钟 | 活动标签或兴趣标签 |
| `campushub:activity:detail:{id}` | String(JSON) | 1-5 分钟 | 活动详情缓存 |
| `campushub:activity:list:{hash}` | String(JSON) | 30-60 秒 | 活动列表查询结果 |
| `campushub:activity:hot` | ZSet | 5-10 分钟 | 热门活动 |
| `campushub:activity:view-dedup:{id}:{viewer}` | String | 1-24 小时 | 浏览量去重 |
| `campushub:counter:activity:view:{id}` | Hash/Counter | 1-5 分钟 | 浏览量临时计数 |

### 4.4 报名与票据

| Key | 类型 | TTL | 说明 |
|---|---|---|---|
| `campushub:registration:lock:{activity_id}` | String | 5-10 秒 | 报名名额锁 |
| `campushub:ticket:{code}` | String(JSON) | 1-5 分钟 | 票据查询缓存 |
| `campushub:ticket:verify:rate:{user_id}` | Counter | 1 分钟 | 核销接口限流 |

报名名额、票据最终状态不能只依赖缓存，必须以 MySQL 事务结果为准。

### 4.5 WebSocket 与在线状态

| Key | 类型 | TTL | 说明 |
|---|---|---|---|
| `campushub:ws:online:{user_id}` | Hash | 60-90 秒，自动续期 | 用户在线状态 |
| `campushub:ws:user-conns:{user_id}` | Set | 60-90 秒 | 用户连接集合 |
| `campushub:ws:conn:{connection_id}` | Hash | 60-90 秒 | 连接元数据 |
| `campushub:ws:group-conns:{group_id}` | Set | 60-90 秒 | 群房间连接集合 |
| `campushub:chat:unread:{user_id}` | Hash | 长期，刷新 | 各群未读数量 |

### 4.6 分布式锁

| Key | TTL | 说明 |
|---|---|---|
| `campushub:lock:cron:activity-status` | 1-5 分钟 | 活动状态定时任务锁 |
| `campushub:lock:cron:verification-timeout` | 1-5 分钟 | 认证超时任务锁 |
| `campushub:lock:cron:outbox-relay` | 30 秒 | Outbox 投递任务锁 |
| `campushub:lock:reindex:{index}` | 10-30 分钟 | 索引重建锁 |

锁值必须包含唯一 Owner 标识，释放时使用 Lua 脚本校验，避免误删其他实例的锁。

### 4.7 通知

| Key | 类型 | TTL | 说明 |
|---|---|---|---|
| `campushub:notification:unread:{user_id}` | String/Counter | 长期，刷新 | 未读通知数量 |
| `campushub:notification:list:{user_id}:{hash}` | String(JSON) | 30-60 秒 | 通知列表缓存 |

## 5. Redis 数据结构使用建议

| 数据结构 | 使用场景 |
|---|---|
| String | 验证码、Token 黑名单、简单计数 |
| Hash | 刷新令牌、连接元数据、在线状态 |
| Set | 用户连接集合、群连接集合 |
| ZSet | 热门活动、排行榜、排序统计 |
| List | 简单排队，不应用于核心消息队列 |
| Stream | 可用于临时内部流，但新系统主异步通道统一为 Kafka |
| Lua | 原子解锁、限流、计数和状态更新 |

## 6. Ristretto 本地缓存设计

### 6.1 版本与依赖

使用：

```text
github.com/dgraph-io/ristretto/v2
```

通过项目内部接口封装，避免业务代码直接依赖 Ristretto。

### 6.2 本地缓存对象

| 对象 | 建议 TTL | 原因 |
|---|---|---|
| 分类列表 | 1-5 分钟 | 低频变更、读取频繁 |
| 标签列表 | 1-5 分钟 | 低频变更、发布活动和兴趣选择常用 |
| 系统配置 | 30 秒-5 分钟 | 减少重复读取 |
| 用户公开基础信息 | 30-60 秒 | 聊天展示需要 |
| 热门活动列表 | 15-60 秒 | 极热点、允许短暂延迟 |
| ES 索引元数据 | 1-5 分钟 | 索引别名变化少 |

### 6.3 不应进入本地缓存的数据

- 报名名额；
- 票据是否已核销；
- 用户密码和认证敏感信息；
- 用户注销状态；
- 支付或强审计状态；
- 需要写入后立即强一致读取的数据。

### 6.4 容量配置

建议配置：

- 最大条目数或内存成本；
- 单条缓存大小限制；
- TTL 上限；
- 关闭或限制 nil 结果缓存；
- 指标统计命中率。

具体容量应在压测后调整，初始应保守设置。

### 6.5 本地缓存失效

数据变更后：

1. Service 在 MySQL 事务中提交业务数据；
2. 写入 Outbox 或发布缓存失效事件；
3. 所有后端实例消费失效事件；
4. 删除本地缓存和对应 Redis 缓存；
5. 下一次请求回源并重建缓存。

失效事件应包含：

- 缓存域；
- Key 模板或资源 ID；
- 数据版本；
- 事件 ID。

## 7. 缓存一致性模式

### 7.1 Cache Aside

适用于活动详情、分类、标签、通知列表。

读取：

1. 先查缓存；
2. 未命中则查 MySQL；
3. 写入 Redis；
4. 返回数据。

更新：

1. 更新 MySQL；
2. 删除缓存；
3. 下次查询重建。

### 7.2 Read/Write Through

不建议在一期自定义复杂 Write Through 层，避免增加开发成本。

### 7.3 延迟双删

可用于极热点数据：

1. 更新 MySQL；
2. 删除缓存；
3. 短暂延迟后再次删除缓存。

但延迟双删不是强一致方案，最终仍依赖版本号和 TTL。

### 7.4 版本号控制

对活动详情、标签列表等可增加版本：

- Redis 保存版本；
- 本地缓存携带版本；
- 版本不一致则删除本地缓存；
- TTL 作为最终兜底。

## 8. 缓存典型问题处理

### 8.1 缓存穿透

对于不存在的数据：

- 缓存空值，短 TTL；
- 使用布隆过滤器，可选；
- 参数合法性校验；
- 对异常请求限流。

### 8.2 缓存击穿

热点 Key 失效时：

- 使用 `singleflight` 合并相同请求；
- 加分布式锁；
- 热点数据逻辑不过期，由后台异步刷新。

### 8.3 缓存雪崩

- TTL 增加随机抖动；
- 不同业务分散过期时间；
- 多级缓存；
- Redis 高可用；
- 限流和降级。

### 8.4 热点写入

浏览量等高频写入不直接更新 MySQL：

1. Redis 去重；
2. Redis 计数；
3. 定时任务批量落库；
4. Outbox/Kafka 可用于异步统计。

## 9. 缓存降级

| 场景 | 处理方式 |
|---|---|
| Redis 短暂不可用 | 本地缓存继续提供部分读能力，未命中直接访问 MySQL |
| 本地缓存失效 | 请求 Redis |
| Redis 和本地缓存均未命中 | 查询 MySQL，并受 singleflight 保护 |
| MySQL 压力升高 | 限流、降级非核心接口、缩短缓存重建范围 |
| 缓存数据版本异常 | 删除缓存并触发回源 |

## 10. 监控指标

需要监控：

- Redis 连接数、内存、CPU、命中率；
- Key 过期和淘汰数量；
- 慢查询；
- 分布式锁获取失败次数；
- 验证码发送和校验次数；
- Ristretto 命中率、驱逐数量；
- 缓存回源次数；
- 单飞合并请求数量。

## 11. 设计约束

1. 每个缓存 Key 必须有负责人、用途和 TTL。
2. 不设置永久业务缓存，永久有效仅限极少数明确的静态数据。
3. 缓存值必须可序列化、可版本化。
4. 缓存不能替代数据库事务。
5. 本地缓存只保存可接受短暂不一致的数据。
6. 缓存失效事件必须幂等。
7. 所有缓存异常不能导致错误数据被长期使用。

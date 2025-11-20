# 🚀 long-gate: A High-Performance Go API Gateway

**long-gate** 是一个基于 Go 语言构建的、高性能、可扩展的开源 API 网关。它被设计用于统一管理、保护和路由您的微服务和后端 API 流量。

它专注于提供强大的请求转发能力（HTTP/RPC/WebSocket）以及基于插件的热插拔中间件功能。

## ✨ 核心特性

- **动态路由管理**: 运行时增删改查路由，无需重启
- **灵活匹配规则**: 支持路径前缀/精确/正则、HTTP 方法、请求头、域名等多维度匹配
- **多种负载均衡**: Round-Robin、加权、最少连接、IP Hash、随机
- **健康检查**: 主动探测后端节点状态，自动摘除不健康节点
- **中间件系统**: 可插拔的洋葱模型，支持日志、CORS、超时等
- **配置持久化**: 基于 ETCD 的分布式配置存储
- **热更新**: 配置变更自动同步，原子替换路由表
- **管理 API**: RESTful 接口管理路由和上游服务

## 📦 快速开始

### 1. 安装依赖

```bash
# 启动 ETCD (使用 Docker)
docker run -d --name etcd \
  -p 2379:2379 \
  -p 2380:2380 \
  quay.io/coreos/etcd:latest \
  /usr/local/bin/etcd \
  --advertise-client-urls http://0.0.0.0:2379 \
  --listen-client-urls http://0.0.0.0:2379

# 安装 Go 依赖
go mod download
```

### 2. 启动网关

```bash
go run cmd/server/main.go
```

- **数据面**: `http://localhost:8080` (处理业务流量)
- **管理 API**: `http://localhost:9000` (配置管理)

## 🔧 配置管理

### 创建上游服务

```bash
curl -X POST http://localhost:9000/admin/upstreams \
  -H "Content-Type: application/json" \
  -d '{
    "id": "user-service",
    "name": "用户服务",
    "type": "round-robin",
    "targets": [
      {"address": "192.168.1.10:8080", "weight": 1},
      {"address": "192.168.1.11:8080", "weight": 1}
    ],
    "health_check": {
      "enabled": true,
      "type": "http",
      "path": "/health",
      "interval": 10,
      "timeout": 5,
      "healthy_threshold": 2,
      "unhealthy_threshold": 3
    }
  }'
```

### 创建路由

```bash
curl -X POST http://localhost:9000/admin/routes \
  -H "Content-Type: application/json" \
  -d '{
    "id": "user-api-route",
    "name": "用户 API",
    "priority": 100,
    "status": 1,
    "predicates": {
      "path": "/api/users",
      "path_type": "prefix",
      "methods": ["GET", "POST"]
    },
    "upstream_id": "user-service"
  }'
```

### 路由匹配示例

#### 1. 前缀匹配（推荐）

```json
{
  "predicates": {
    "path": "/api/v1",
    "path_type": "prefix"
  }
}
```

匹配: `/api/v1/users`, `/api/v1/orders`

#### 2. 精确匹配

```json
{
  "predicates": {
    "path": "/health",
    "path_type": "exact"
  }
}
```

仅匹配: `/health`

#### 3. 正则匹配

```json
{
  "predicates": {
    "path": "^/api/users/\\d+$",
    "path_type": "regex"
  }
}
```

匹配: `/api/users/123`, `/api/users/456`

#### 4. 参数化路由

```json
{
  "predicates": {
    "path": "/api/users/:id",
    "path_type": "prefix"
  }
}
```

匹配: `/api/users/123` → `params["id"] = "123"`

#### 5. 多条件组合

```json
{
  "predicates": {
    "path": "/admin",
    "path_type": "prefix",
    "methods": ["GET", "POST"],
    "headers": {
      "X-API-Key": "secret"
    },
    "hosts": ["admin.example.com"]
  }
}
```

## 🔀 负载均衡策略

### Round-Robin (轮询)

```json
{"type": "round-robin"}
```

依次分配请求到每个节点，适合节点性能一致的场景。

### Weighted (加权)

```json
{
  "type": "weighted",
  "targets": [
    {"address": "server1:8080", "weight": 3},
    {"address": "server2:8080", "weight": 1}
  ]
}
```

按权重分配，权重越高分配越多请求。

### Least Connection (最少连接)

```json
{"type": "least-conn"}
```

选择当前活跃连接数最少的节点。

### IP Hash (IP 哈希)

```json
{"type": "ip-hash"}
```

根据客户端 IP 哈希，同一 IP 始终路由到同一节点（会话保持）。

### Random (随机)

```json
{"type": "random"}
```

随机选择节点。

## 🏥 健康检查

网关会定期检查后端节点健康状态：

- **检查类型**: HTTP / TCP
- **检查间隔**: 可配置 (默认 10 秒)
- **健康阈值**: 连续成功 N 次标记为健康
- **不健康阈值**: 连续失败 N 次标记为不健康

不健康的节点会自动从负载均衡中摘除。

## 🔌 中间件

### 内置中间件

- **Recovery**: 捕获 panic，防止进程崩溃
- **Logger**: 记录请求日志（路径、耗时、状态码）
- **CORS**: 跨域支持
- **RequestID**: 为每个请求生成唯一 ID
- **Timeout**: 请求超时控制

### 自定义中间件

```go
func RateLimitMiddleware(limit int) middleware.Middleware {
    limiter := rate.NewLimiter(rate.Limit(limit), limit)
    
    return func(next middleware.HandlerFunc) middleware.HandlerFunc {
        return func(ctx *middleware.Context) {
            if !limiter.Allow() {
                ctx.Response.WriteHeader(http.StatusTooManyRequests)
                ctx.Abort()
                return
            }
            next(ctx)
        }
    }
}
```

## 📊 管理 API 文档

### 路由管理

| 方法   | 路径                | 说明         |
| ------ | ------------------- | ------------ |
| GET    | `/admin/routes`     | 获取所有路由 |
| POST   | `/admin/routes`     | 创建路由     |
| GET    | `/admin/routes/:id` | 获取单个路由 |
| PUT    | `/admin/routes/:id` | 更新路由     |
| DELETE | `/admin/routes/:id` | 删除路由     |

### 上游管理

| 方法   | 路径                   | 说明         |
| ------ | ---------------------- | ------------ |
| GET    | `/admin/upstreams`     | 获取所有上游 |
| POST   | `/admin/upstreams`     | 创建上游     |
| GET    | `/admin/upstreams/:id` | 获取单个上游 |
| PUT    | `/admin/upstreams/:id` | 更新上游     |
| DELETE | `/admin/upstreams/:id` | 删除上游     |

### 健康检查

| 方法 | 路径            | 说明         |
| ---- | --------------- | ------------ |
| GET  | `/admin/health` | 网关健康状态 |

## 🏗️ 架构亮点

### 1. 原子更新路由

使用 `atomic.Value` 实现无锁路由表切换：

```go
// 构建新路由表
newTable := &RouteTable{routes: newRoutes}

// 原子替换
router.routes.Store(newTable)
```

### 2. 增量更新优化

单个路由变更时，仅重建索引而非全量加载：

```go
func (r *Router) AddRoute(route *config.Route) {
    // 复制现有路由 + 新路由
    newRoutes := append(oldRoutes, route)
    // 原子替换
    r.routes.Store(newTable)
}
```

### 3. ETCD Watch 断线重连

```go
watchChan := client.Watch(ctx, prefix, clientv3.WithPrefix())
for watchResp := range watchChan {
    if watchResp.Err() != nil {
        // 重新建立 Watch
        time.Sleep(5 * time.Second)
        watchChan = client.Watch(ctx, prefix, clientv3.WithPrefix())
    }
}
```

## 📈 性能优化建议

1. **路由优先级**: 高频路由设置更高优先级，减少匹配次数
2. **连接池**: 使用 `http.Transport` 配置连接池参数
3. **日志异步**: 使用 Zap 的异步日志模式
4. **缓存**: 为静态路由增加 LRU 缓存
5. **批量操作**: ETCD 写入使用事务批量提交

## 🔒 安全建议

- 管理 API 添加认证（JWT / API Key）
- ETCD 启用 TLS 加密
- 限流中间件防止 DDoS
- 敏感配置使用加密存储

## 🧪 测试

```bash
# 单元测试
go test ./...

# 压力测试
ab -n 10000 -c 100 http://localhost:8080/api/test
```

## 📝 TODO

- [ ] 实现 gRPC 反向代理
- [ ] 增加 Prometheus 指标导出
- [ ] WebSocket 支持
- [ ] 流量镜像功能
- [ ] 灰度发布策略
- [ ] 分布式限流（基于 Redis）

## 💡 贡献指南
我们非常欢迎社区贡献！请参阅 [CONTRIBUTING.md](CONTRIBUTING.md) 了解如何提交 Bug 报告和 Pull Request。

## 📄 许可证
本项目采用 MIT 许可证，详情请参阅 [LICENSE](LICENSE) 文件。
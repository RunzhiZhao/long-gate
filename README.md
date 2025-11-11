# 🚀 long-gate: A High-Performance Go API Gateway

**long-gate** 是一个基于 Go 语言构建的、高性能、可扩展的开源 API 网关。它被设计用于统一管理、保护和路由您的微服务和后端 API 流量。

它专注于提供强大的请求转发能力（HTTP/RPC/WebSocket）以及基于插件的热插拔中间件功能。

## ✨ 主要特性 (MVP)

* **高性能转发:** 基于 Go 协程（Goroutine）和标准库的反向代理，支持高性能的 HTTP、WebSocket 请求转发。
* **动态路由:** 支持基于 Path、Host 的路由匹配，配置实时更新，无需重启。
* **统一鉴权:** 内置 JWT (JSON Web Token) 鉴权中间件，保护您的后端服务。
* **流量控制:** 内置令牌桶算法的限流（Rate Limit）中间件，保障服务稳定。
* **配置驱动:** 路由和服务配置通过 YAML 文件管理，结构清晰。

## 🛠️ 技术栈

| 模块         | 核心技术/库                                 |
| :----------- | :------------------------------------------ |
| **基础框架** | Go 标准库 (`net/http`)                      |
| **配置管理** | `gopkg.in/yaml.v3`, `fsnotify` (实现热重载) |
| **反向代理** | `net/http/httputil`                         |
| **JWT 鉴权** | `github.com/golang-jwt/jwt`                 |
| **限流**     | `golang.org/x/time/rate`                    |

## 🚀 快速开始

### 1. 克隆仓库

```bash
git clone [https://github.com/your-github-username/long-gate.git](https://github.com/your-github-username/long-gate.git)
cd long-gate
```

### 2. 配置路由

修改 configs/gateway.yaml 文件，定义您的上游服务和路由规则。

```YAML

# 示例: 转发到本地 8081 端口的服务
services:
  user-service:
    addr: "http://localhost:8081" 
    type: "http"

routes:
  - path: "/api/v1/user"
    service_id: "user-service"
    middlewares:
      - name: "jwt"  # 开启 JWT 鉴权
      - name: "rate_limit"
        param: "100/s" # 每秒 100 次请求
```

### 3. 运行网关

```bash
go run cmd/main.go
```

默认情况下，long-gate 将监听 http://localhost:8080。

## 💡 贡献指南
我们非常欢迎社区贡献！请参阅 [CONTRIBUTING.md] 了解如何提交 Bug 报告和 Pull Request。

## 📄 许可证
本项目采用 MIT 许可证，详情请参阅 [LICENSE](LICENSE) 文件。
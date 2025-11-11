package config

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// Config 定义整个网关的配置结构
type Config struct {
	Gateway  GatewayConfig            `yaml:"gateway"`
	Services map[string]ServiceConfig `yaml:"services"`
	Routes   []RouteConfig            `yaml:"routes"`
}

// GatewayConfig 定义网关自身配置
type GatewayConfig struct {
	Port      int    `yaml:"port"`
	JWTSecret string `yaml:"jwt_secret"`
}

// ServiceConfig 定义上游服务配置
type ServiceConfig struct {
	Addr string `yaml:"addr"` // 后端服务地址，如 http://localhost:8081
	Type string `yaml:"type"` // 代理类型: http, rpc
}

// RouteConfig 定义路由规则配置
type RouteConfig struct {
	ID          string             `yaml:"id"`
	Path        string             `yaml:"path"` // 匹配路径
	ServiceID   string             `yaml:"service_id"`
	Middlewares []MiddlewareConfig `yaml:"middlewares"`
}

// MiddlewareConfig 定义中间件配置
type MiddlewareConfig struct {
	Name  string `yaml:"name"`
	Param string `yaml:"param,omitempty"` // 可选参数，如限流速率
}

var (
	currentConfig *Config
	configMutex   sync.RWMutex // 读写锁，保证热重载时的并发安全
)

// LoadAndWatchConfig 加载配置并启动文件监听
func LoadAndWatchConfig(path string) error {
	if err := loadConfig(path); err != nil {
		return err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	go watch(watcher, path)
	return watcher.Add(path)
}

// loadConfig 从文件加载配置到内存
func loadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 原子更新配置
	configMutex.Lock()
	currentConfig = &cfg
	configMutex.Unlock()

	// 🔔 通知 Router 模块更新路由表
	// ⚠️ 实际代码中需要在这里调用 router.UpdateRoutes(cfg.Routes)
	// 为了避免循环依赖，我们暂时将 router 逻辑放在 router.HandleRequest 中读取配置
	// 生产环境中，建议通过 Channel 或 Callback 机制解耦。

	log.Println("Configuration loaded/reloaded successfully.")
	return nil
}

// watch 监听配置文件变化
func watch(watcher *fsnotify.Watcher, path string) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			// 仅在文件写入或重命名时触发重载
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Rename) {
				// 延迟加载，防止编辑器频繁保存导致的多次重载
				time.Sleep(100 * time.Millisecond)
				log.Printf("Config file modified: %s. Reloading...", path)
				if err := loadConfig(path); err != nil {
					log.Printf("Error reloading config: %v", err)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("error:", err)
		}
	}
}

// GetRoutes 获取当前的路由配置（读安全）
func GetRoutes() []RouteConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	if currentConfig == nil {
		return nil
	}
	// 返回副本防止外部修改
	return currentConfig.Routes
}

// GetServiceConfig 获取当前的服务配置
func GetServiceConfig(id string) (ServiceConfig, bool) {
	configMutex.RLock()
	defer configMutex.RUnlock()
	if currentConfig == nil {
		return ServiceConfig{}, false
	}
	svc, ok := currentConfig.Services[id]
	return svc, ok
}

// GetGatewayConfig 获取网关自身配置
func GetGatewayConfig() GatewayConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	if currentConfig == nil {
		return GatewayConfig{}
	}
	return currentConfig.Gateway
}

package config

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// LoadBalanceType 负载均衡类型
type LoadBalanceType string

const (
	LoadBalanceRoundRobin LoadBalanceType = "round-robin"
	LoadBalanceWeighted   LoadBalanceType = "weighted"
	LoadBalanceLeastConn  LoadBalanceType = "least-conn"
	LoadBalanceIPHash     LoadBalanceType = "ip-hash"
	LoadBalanceRandom     LoadBalanceType = "random"
)

// Upstream 上游服务定义
type Upstream struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Type        LoadBalanceType `json:"type"`
	Targets     []*Target       `json:"targets"`
	HealthCheck *HealthCheck    `json:"health_check,omitempty"`
	Timeout     int             `json:"timeout"` // 请求超时(秒)
	Retries     int             `json:"retries"` // 重试次数
	Version     int64           `json:"version"`
	CreateTime  int64           `json:"create_time"`
	UpdateTime  int64           `json:"update_time"`

	mu sync.RWMutex // 保护 Targets 状态变更
}

// Target 后端节点
type Target struct {
	Address  string            `json:"address"` // host:port
	Weight   int               `json:"weight"`  // 权重(1-100)
	Status   TargetStatus      `json:"status"`
	Metadata map[string]string `json:"metadata,omitempty"`

	// 健康检查相关
	FailCount   int       `json:"-"` // 连续失败次数
	LastCheckAt time.Time `json:"-"`
	LastFailAt  time.Time `json:"-"`

	// 连接数统计 (用于 least-conn)
	ActiveConns int `json:"-"`
}

// TargetStatus 节点状态
type TargetStatus string

const (
	TargetStatusHealthy   TargetStatus = "healthy"
	TargetStatusUnhealthy TargetStatus = "unhealthy"
	TargetStatusUnknown   TargetStatus = "unknown"
)

// HealthCheck 健康检查配置
type HealthCheck struct {
	Enabled            bool   `json:"enabled"`
	Type               string `json:"type"`                // http/tcp/grpc
	Path               string `json:"path"`                // HTTP 检查路径
	Interval           int    `json:"interval"`            // 检查间隔(秒)
	Timeout            int    `json:"timeout"`             // 超时时间(秒)
	HealthyThreshold   int    `json:"healthy_threshold"`   // 健康阈值
	UnhealthyThreshold int    `json:"unhealthy_threshold"` // 不健康阈值
}

// Validate 验证上游配置
func (u *Upstream) Validate() error {
	if u.ID == "" {
		return fmt.Errorf("upstream id cannot be empty")
	}
	if len(u.Targets) == 0 {
		return fmt.Errorf("upstream must have at least one target")
	}

	// 验证负载均衡类型
	validTypes := map[LoadBalanceType]bool{
		LoadBalanceRoundRobin: true,
		LoadBalanceWeighted:   true,
		LoadBalanceLeastConn:  true,
		LoadBalanceIPHash:     true,
		LoadBalanceRandom:     true,
	}
	if !validTypes[u.Type] {
		return fmt.Errorf("invalid load balance type: %s", u.Type)
	}

	// 验证 Targets
	for i, target := range u.Targets {
		if target.Address == "" {
			return fmt.Errorf("target[%d] address cannot be empty", i)
		}
		if target.Weight < 1 {
			target.Weight = 1 // 默认权重
		}
		if target.Status == "" {
			target.Status = TargetStatusUnknown
		}
	}

	// 健康检查默认值
	if u.HealthCheck != nil && u.HealthCheck.Enabled {
		if u.HealthCheck.Interval == 0 {
			u.HealthCheck.Interval = 10
		}
		if u.HealthCheck.Timeout == 0 {
			u.HealthCheck.Timeout = 5
		}
		if u.HealthCheck.HealthyThreshold == 0 {
			u.HealthCheck.HealthyThreshold = 2
		}
		if u.HealthCheck.UnhealthyThreshold == 0 {
			u.HealthCheck.UnhealthyThreshold = 3
		}
	}

	return nil
}

// GetHealthyTargets 获取健康的节点列表
func (u *Upstream) GetHealthyTargets() []*Target {
	u.mu.RLock()
	defer u.mu.RUnlock()

	healthy := make([]*Target, 0, len(u.Targets))
	for _, target := range u.Targets {
		if target.Status == TargetStatusHealthy {
			healthy = append(healthy, target)
		}
	}
	return healthy
}

// UpdateTargetStatus 更新节点状态
func (u *Upstream) UpdateTargetStatus(address string, status TargetStatus) {
	u.mu.Lock()
	defer u.mu.Unlock()

	for _, target := range u.Targets {
		if target.Address == address {
			target.Status = status
			target.LastCheckAt = time.Now()
			return
		}
	}
}

// IncrementActiveConns 增加活跃连接数
func (u *Upstream) IncrementActiveConns(address string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	for _, target := range u.Targets {
		if target.Address == address {
			target.ActiveConns++
			return
		}
	}
}

// DecrementActiveConns 减少活跃连接数
func (u *Upstream) DecrementActiveConns(address string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	for _, target := range u.Targets {
		if target.Address == address && target.ActiveConns > 0 {
			target.ActiveConns--
			return
		}
	}
}

// ToJSON 序列化为 JSON
func (u *Upstream) ToJSON() ([]byte, error) {
	return json.Marshal(u)
}

// FromJSON 从 JSON 反序列化
func (u *Upstream) FromJSON(data []byte) error {
	if err := json.Unmarshal(data, u); err != nil {
		return err
	}
	return u.Validate()
}

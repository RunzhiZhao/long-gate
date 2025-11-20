package balancer

import (
	"errors"
	"hash/crc32"
	"math/rand"
	"sync"
	"sync/atomic"

	"github.com/RunzhiZhao/long-gate/internal/config"
)

var (
	ErrNoHealthyTarget = errors.New("no healthy target available")
)

// LoadBalancer 负载均衡器接口
type LoadBalancer interface {
	Select(clientIP string) (*config.Target, error)
	UpdateTargets(targets []*config.Target)
}

// NewLoadBalancer 创建负载均衡器
func NewLoadBalancer(lbType config.LoadBalanceType, upstream *config.Upstream) LoadBalancer {
	switch lbType {
	case config.LoadBalanceRoundRobin:
		return NewRoundRobinBalancer(upstream)
	case config.LoadBalanceWeighted:
		return NewWeightedBalancer(upstream)
	case config.LoadBalanceLeastConn:
		return NewLeastConnBalancer(upstream)
	case config.LoadBalanceIPHash:
		return NewIPHashBalancer(upstream)
	case config.LoadBalanceRandom:
		return NewRandomBalancer(upstream)
	default:
		return NewRoundRobinBalancer(upstream)
	}
}

// --- Round Robin 轮询 ---

type RoundRobinBalancer struct {
	upstream *config.Upstream
	current  uint32
}

func NewRoundRobinBalancer(upstream *config.Upstream) *RoundRobinBalancer {
	return &RoundRobinBalancer{
		upstream: upstream,
		current:  0,
	}
}

func (rb *RoundRobinBalancer) Select(clientIP string) (*config.Target, error) {
	targets := rb.upstream.GetHealthyTargets()
	if len(targets) == 0 {
		return nil, ErrNoHealthyTarget
	}

	idx := atomic.AddUint32(&rb.current, 1) % uint32(len(targets))
	return targets[idx], nil
}

func (rb *RoundRobinBalancer) UpdateTargets(targets []*config.Target) {
	// Round Robin 不需要特殊更新逻辑
}

// --- Weighted 加权轮询 ---

type WeightedBalancer struct {
	upstream *config.Upstream
	current  int
	mu       sync.Mutex
}

func NewWeightedBalancer(upstream *config.Upstream) *WeightedBalancer {
	return &WeightedBalancer{
		upstream: upstream,
		current:  0,
	}
}

func (wb *WeightedBalancer) Select(clientIP string) (*config.Target, error) {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	targets := wb.upstream.GetHealthyTargets()
	if len(targets) == 0 {
		return nil, ErrNoHealthyTarget
	}

	// 计算总权重
	totalWeight := 0
	for _, target := range targets {
		totalWeight += target.Weight
	}

	// 加权选择
	wb.current = (wb.current + 1) % totalWeight
	sum := 0
	for _, target := range targets {
		sum += target.Weight
		if wb.current < sum {
			return target, nil
		}
	}

	return targets[0], nil
}

func (wb *WeightedBalancer) UpdateTargets(targets []*config.Target) {
	wb.mu.Lock()
	wb.current = 0
	wb.mu.Unlock()
}

// --- Least Connection 最少连接 ---

type LeastConnBalancer struct {
	upstream *config.Upstream
}

func NewLeastConnBalancer(upstream *config.Upstream) *LeastConnBalancer {
	return &LeastConnBalancer{
		upstream: upstream,
	}
}

func (lb *LeastConnBalancer) Select(clientIP string) (*config.Target, error) {
	targets := lb.upstream.GetHealthyTargets()
	if len(targets) == 0 {
		return nil, ErrNoHealthyTarget
	}

	// 选择连接数最少的节点
	minConns := targets[0].ActiveConns
	selected := targets[0]

	for _, target := range targets[1:] {
		if target.ActiveConns < minConns {
			minConns = target.ActiveConns
			selected = target
		}
	}

	return selected, nil
}

func (lb *LeastConnBalancer) UpdateTargets(targets []*config.Target) {
	// Least Connection 不需要特殊更新逻辑
}

// --- IP Hash ---

type IPHashBalancer struct {
	upstream *config.Upstream
}

func NewIPHashBalancer(upstream *config.Upstream) *IPHashBalancer {
	return &IPHashBalancer{
		upstream: upstream,
	}
}

func (ih *IPHashBalancer) Select(clientIP string) (*config.Target, error) {
	targets := ih.upstream.GetHealthyTargets()
	if len(targets) == 0 {
		return nil, ErrNoHealthyTarget
	}

	// 使用 CRC32 哈希客户端 IP
	hash := crc32.ChecksumIEEE([]byte(clientIP))
	idx := int(hash) % len(targets)
	return targets[idx], nil
}

func (ih *IPHashBalancer) UpdateTargets(targets []*config.Target) {
	// IP Hash 不需要特殊更新逻辑
}

// --- Random 随机 ---

type RandomBalancer struct {
	upstream *config.Upstream
}

func NewRandomBalancer(upstream *config.Upstream) *RandomBalancer {
	return &RandomBalancer{
		upstream: upstream,
	}
}

func (rb *RandomBalancer) Select(clientIP string) (*config.Target, error) {
	targets := rb.upstream.GetHealthyTargets()
	if len(targets) == 0 {
		return nil, ErrNoHealthyTarget
	}

	idx := rand.Intn(len(targets))
	return targets[idx], nil
}

func (rb *RandomBalancer) UpdateTargets(targets []*config.Target) {
	// Random 不需要特殊更新逻辑
}

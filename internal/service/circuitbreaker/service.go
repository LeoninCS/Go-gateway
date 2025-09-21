package circuitbreaker

import (
	"errors"
	"log"
	"sync"
	"time"
)

// 全局错误定义
var (
	ErrOpenState       = errors.New("circuit breaker is open")              // 熔断器处于打开状态
	ErrTooManyRequests = errors.New("too many requests")                    // 请求数超过限制（预留）
	ErrServiceNotFound = errors.New("service not found in circuit breaker") // 服务未找到
)

// State 熔断器状态枚举
type State int

const (
	StateClosed   State = iota // 关闭状态：允许请求，记录失败数
	StateOpen                  // 打开状态：拒绝请求，等待超时后进入半开
	StateHalfOpen              // 半开状态：允许少量请求试探，成功则关闭，失败则重新打开
)

// GetState 将状态转为可读字符串
func (s State) GetState() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitState 熔断器状态的对外展示结构（用于监控、日志等）
type CircuitState struct {
	ServiceName      string    `json:"service_name"`             // 服务名
	State            string    `json:"state"`                    // 状态（字符串形式）
	FailureCount     int       `json:"failure_count"`            // 失败次数
	SuccessCount     int       `json:"success_count"`            // 成功次数
	LastOpenTime     time.Time `json:"last_open_time,omitempty"` // 最后一次打开时间
	FailureThreshold int       `json:"failure_threshold"`        // 失败阈值（达到则打开）
	SuccessThreshold int       `json:"success_threshold"`        // 成功阈值（半开时达到则关闭）
	ResetTimeout     string    `json:"reset_timeout"`            // 重置超时时间（字符串形式）
}

// Service 熔断器服务接口（定义核心能力，解耦实现与调用）
type Service interface {
	CheckCircuit(serviceName string) (bool, error) // 检查是否允许请求
	RecordResult(serviceName string, success bool) // 记录请求结果（成功/失败）
	GetAllState() map[string]CircuitState          // 获取所有服务的熔断器状态
	Reset(serviceName string) error                // 重置指定服务的熔断器
	Close() error                                  // 优雅关闭服务（清理资源）
}

// CircuitBreaker 单个服务的熔断器实例（承载单个服务的状态）
type CircuitBreaker struct {
	mu           sync.Mutex // 保护当前熔断器实例的并发安全
	state        State      // 当前状态
	failureCount int        // 失败次数
	successCount int        // 成功次数（主要用于半开状态）
	lastOpenTime time.Time  // 最后一次进入打开状态的时间
}

// service Service 接口的具体实现（管理多个服务的熔断器）
type service struct {
	mu               sync.RWMutex               // 保护多服务熔断器映射的并发安全
	circuitBreakers  map[string]*CircuitBreaker // 服务名 -> 熔断器实例的映射
	FailureThreshold int                        // 全局失败阈值（默认5次）
	SuccessThreshold int                        // 全局成功阈值（默认2次）
	ResetTimeout     time.Duration              // 全局重置超时时间（默认1分钟）
}

// NewService 创建熔断器服务实例（返回接口类型，隐藏内部实现）
func NewService(failureThreshold int, successThreshold int, resetTimeout time.Duration) Service {
	// 配置默认值（避免传入非法参数）
	if failureThreshold <= 0 {
		failureThreshold = 5
	}
	if successThreshold <= 0 {
		successThreshold = 2
	}
	if resetTimeout <= 0 {
		resetTimeout = 1 * time.Minute
	}

	// 初始化服务实例，创建熔断器映射
	return &service{
		circuitBreakers:  make(map[string]*CircuitBreaker),
		FailureThreshold: failureThreshold,
		SuccessThreshold: successThreshold,
		ResetTimeout:     resetTimeout,
	}
}

// GetAllState 返回所有服务的熔断器状态（对外展示用）
func (s *service) GetAllState() map[string]CircuitState {
	s.mu.RLock() // 读锁：仅查询，不修改映射
	defer s.mu.RUnlock()

	result := make(map[string]CircuitState, len(s.circuitBreakers))
	for serviceName, cb := range s.circuitBreakers {
		cb.mu.Lock() // 锁单个熔断器实例，避免状态读取时被修改
		// 组装对外的状态结构
		result[serviceName] = CircuitState{
			ServiceName:      serviceName,
			State:            cb.state.GetState(),
			FailureCount:     cb.failureCount,
			SuccessCount:     cb.successCount,
			LastOpenTime:     cb.lastOpenTime,
			FailureThreshold: s.FailureThreshold,
			SuccessThreshold: s.SuccessThreshold,
			ResetTimeout:     s.ResetTimeout.String(),
		}
		cb.mu.Unlock()
	}
	return result
}

// Reset 重置指定服务的熔断器（状态归位，计数清零）
func (s *service) Reset(serviceName string) error {
	s.mu.RLock() // 读锁：查询熔断器是否存在
	cb, exists := s.circuitBreakers[serviceName]
	s.mu.RUnlock()

	if !exists {
		return ErrServiceNotFound
	}

	// 重置熔断器内部状态
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failureCount = 0
	cb.successCount = 0
	log.Printf("[熔断器服务] 已重置服务 '%s' 的熔断器", serviceName)
	return nil
}

// CheckCircuit 检查指定服务的熔断器状态，返回是否允许请求
func (s *service) CheckCircuit(serviceName string) (bool, error) {
	// 1. 确保服务的熔断器实例存在（不存在则创建）
	s.mu.Lock()
	cb, exists := s.circuitBreakers[serviceName]
	if !exists {
		cb = &CircuitBreaker{state: StateClosed} // 新熔断器默认处于关闭状态
		s.circuitBreakers[serviceName] = cb
		log.Printf("[熔断器服务] 为服务 '%s' 初始化熔断器", serviceName)
	}
	s.mu.Unlock()

	// 2. 检查熔断器状态，决定是否允许请求
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateOpen:
		// 打开状态：检查是否超过重置超时时间，超时则进入半开
		if time.Since(cb.lastOpenTime) > s.ResetTimeout {
			cb.state = StateHalfOpen
			cb.failureCount = 0
			cb.successCount = 0
			log.Printf("[熔断器服务] 服务 '%s' 熔断器从 open 转为 half-open", serviceName)
			return true, nil // 半开状态允许试探请求
		}
		// 未超时：拒绝请求
		return false, ErrOpenState

	case StateHalfOpen:
		// 半开状态：允许请求（试探）
		return true, nil

	case StateClosed:
		// 关闭状态：允许请求
		return true, nil

	default:
		// 未知状态：默认允许请求（降级策略）
		log.Printf("[熔断器服务] 服务 '%s' 熔断器状态未知，默认允许请求", serviceName)
		return true, nil
	}
}

// RecordResult 记录指定服务的请求结果，更新熔断器状态
func (s *service) RecordResult(serviceName string, success bool) {
	// 1. 检查服务的熔断器是否存在（不存在则忽略，避免无意义操作）
	s.mu.RLock()
	cb, exists := s.circuitBreakers[serviceName]
	s.mu.RUnlock()

	if !exists {
		log.Printf("[熔断器服务] 服务 '%s' 未初始化熔断器，忽略结果记录", serviceName)
		return
	}

	// 2. 根据请求结果更新熔断器状态
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if success {
		// 成功场景：处理半开状态的成功计数
		cb.successCount++
		log.Printf("[熔断器服务] 服务 '%s' 请求成功，当前成功计数: %d", serviceName, cb.successCount)

		// 半开状态下，成功次数达到阈值则转为关闭
		if cb.state == StateHalfOpen && cb.successCount >= s.SuccessThreshold {
			cb.state = StateClosed
			cb.failureCount = 0
			cb.successCount = 0
			log.Printf("[熔断器服务] 服务 '%s' 熔断器从 half-open 转为 closed", serviceName)
		}

	} else {
		// 失败场景：处理关闭/半开状态的失败计数
		cb.failureCount++
		log.Printf("[熔断器服务] 服务 '%s' 请求失败，当前失败计数: %d", serviceName, cb.failureCount)

		// 关闭状态下，失败次数达到阈值则转为打开
		if cb.state == StateClosed && cb.failureCount >= s.FailureThreshold {
			cb.state = StateOpen
			cb.lastOpenTime = time.Now()
			log.Printf("[熔断器服务] 服务 '%s' 熔断器从 closed 转为 open", serviceName)
		}

		// 半开状态下，只要失败就立即转为打开
		if cb.state == StateHalfOpen {
			cb.state = StateOpen
			cb.lastOpenTime = time.Now()
			log.Printf("[熔断器服务] 服务 '%s' 熔断器从 half-open 转为 open", serviceName)
		}
	}
}

// Close 优雅关闭熔断器服务（清理资源，此处无长期后台任务，主要用于日志和扩展）
func (s *service) Close() error {
	log.Println("[熔断器服务] 开始优雅关闭...")
	// 若后续添加了后台任务（如定期清理过期熔断器），可在此处通过 context 取消任务
	log.Println("[熔断器服务] 已成功关闭")
	return nil
}

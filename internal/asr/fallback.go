package asr

import (
	"strings"
	"sync"
	"time"

	"github.com/iabetor/pibuddy/internal/logger"
)

// FallbackEngine 多层兜底 ASR 引擎。
// 支持按优先级尝试多个引擎，失败时自动切换到下一个。
// 引擎优先级由配置文件 asr.priority 列表决定，
// sherpa 始终作为最终兜底引擎（端点检测 + 离线识别）。
type FallbackEngine struct {
	engines    []Engine       // 引擎列表（按优先级排序）
	engineType []EngineType   // 引擎类型（用于日志）
	mu         sync.RWMutex   // 保护当前引擎
	currentIdx int            // 当前使用的引擎索引
	failedAt   map[int]time.Time // 引擎失败时间

	// 恢复机制
	recoveryInterval time.Duration // 尝试恢复在线引擎的间隔
	lastRecoveryTry  time.Time     // 上次尝试恢复的时间

	// 端点检测：最后一个引擎（通常是 sherpa）用于检测端点
	endpointDetectorIdx int

	// 端点触发标记：IsEndpoint() 触发后设置，GetResult() 读取后清除
	endpointTriggered bool
}

// FallbackConfig 兜底引擎配置
type FallbackConfig struct {
	// 引擎列表（按优先级排序）
	Engines []Engine
	// 引擎类型（用于日志）
	EngineTypes []EngineType
	// 恢复间隔（默认 5 分钟）
	RecoveryInterval time.Duration
}

// NewFallbackEngine 创建多层兜底引擎。
func NewFallbackEngine(cfg FallbackConfig) *FallbackEngine {
	if len(cfg.Engines) == 0 {
		panic("FallbackEngine: 至少需要一个引擎")
	}
	if len(cfg.Engines) != len(cfg.EngineTypes) {
		panic("FallbackEngine: Engines 和 EngineTypes 长度必须一致")
	}

	recoveryInterval := cfg.RecoveryInterval
	if recoveryInterval == 0 {
		recoveryInterval = 5 * time.Minute
	}

	e := &FallbackEngine{
		engines:             cfg.Engines,
		engineType:          cfg.EngineTypes,
		currentIdx:          0,
		failedAt:            make(map[int]time.Time),
		recoveryInterval:    recoveryInterval,
		endpointDetectorIdx: len(cfg.Engines) - 1, // 最后一个引擎用于端点检测
	}

	// 找到第一个可用引擎
	for i, engine := range e.engines {
		if se, ok := engine.(StatusEngine); ok {
			if se.Status() == StatusAvailable {
				e.currentIdx = i
				break
			}
		} else {
			// 非状态引擎（如 sherpa），默认可用
			e.currentIdx = i
			break
		}
	}

	logger.Infof("[asr] Fallback 引擎已初始化，当前引擎: %s (端点检测: %s)",
		e.engineType[e.currentIdx], e.engineType[e.endpointDetectorIdx])
	return e
}

// switchToNext 切换到下一个可用引擎
func (e *FallbackEngine) switchToNext(currentIdx int, reason string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 记录失败时间
	e.failedAt[currentIdx] = time.Now()

	// 查找下一个可用引擎
	for i := range e.engines {
		if i == currentIdx {
			continue
		}

		engine := e.engines[i]
		engineType := e.engineType[i]

		// 检查引擎状态
		if se, ok := engine.(StatusEngine); ok {
			status := se.Status()
			if status != StatusAvailable {
				continue
			}
		}

		// 切换引擎
		oldType := e.engineType[e.currentIdx]
		e.currentIdx = i
		logEngineSwitch(oldType, engineType, reason)
		return true
	}

	logger.Errorf("[asr] 无可用引擎，所有引擎均已失败")
	return false
}

// tryRecover 尝试恢复到优先级更高的引擎
func (e *FallbackEngine) tryRecover() {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()

	// 检查是否需要尝试恢复
	if now.Sub(e.lastRecoveryTry) < e.recoveryInterval {
		return
	}
	e.lastRecoveryTry = now

	// 从最高优先级开始检查
	for i := 0; i < e.currentIdx; i++ {
		engine := e.engines[i]
		engineType := e.engineType[i]

		// 检查是否在冷却期
		if failedAt, ok := e.failedAt[i]; ok {
			if now.Sub(failedAt) < e.recoveryInterval {
				continue
			}
		}

		// 检查引擎状态
		if se, ok := engine.(StatusEngine); ok {
			status := se.Status()
			if status == StatusAvailable {
				oldType := e.engineType[e.currentIdx]
				e.currentIdx = i
				logger.Infof("[asr] 引擎已恢复: %s -> %s", oldType, engineType)
				return
			}
		}
	}
}

// Feed 实现 Engine 接口。
// 将音频数据同时送入所有引擎，确保切换引擎时新引擎有数据。
func (e *FallbackEngine) Feed(samples []float32) {
	// 尝试恢复
	e.tryRecover()

	// 送入所有引擎（确保切换时新引擎有数据）
	for _, engine := range e.engines {
		engine.Feed(samples)
	}
}

// GetResult 实现 Engine 接口。
// 非端点触发时：返回端点检测引擎（sherpa）的实时中间结果。
// 端点触发时：等待在线批处理引擎返回结果（最多 10 秒），超时则用 sherpa 的结果兜底。
func (e *FallbackEngine) GetResult() string {
	e.mu.RLock()
	currentIdx := e.currentIdx
	endpointIdx := e.endpointDetectorIdx
	isEndpoint := e.endpointTriggered
	e.mu.RUnlock()

	// 如果当前引擎就是端点检测引擎（sherpa 单引擎模式），直接返回
	if currentIdx == endpointIdx {
		return e.engines[endpointIdx].GetResult()
	}

	currentEngine := e.engines[currentIdx]

	// 非端点触发场景：返回 sherpa 的实时中间结果
	if !isEndpoint {
		// 调用批处理引擎的 GetResult() 检查异步结果（可能来自上一次触发）
		if _, ok := currentEngine.(BatchEngine); ok {
			currentEngine.GetResult() // 消费可能存在的异步错误/结果
		}
		return e.engines[endpointIdx].GetResult()
	}

	// 端点触发场景：等待批处理引擎的异步结果
	e.mu.Lock()
	e.endpointTriggered = false
	e.mu.Unlock()

	if _, ok := currentEngine.(BatchEngine); ok {
		// 轮询等待异步结果，最多 10 秒
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			result := currentEngine.GetResult()
			if result != "" {
				return result
			}

			// 检查引擎是否降级（异步调用失败）
			if se, ok := currentEngine.(StatusEngine); ok {
				if se.Status() != StatusAvailable {
					logger.Warnf("[asr] 批处理引擎 %s 不可用，切换引擎", e.engineType[currentIdx])
					if e.switchToNext(currentIdx, "引擎不可用") {
						e.mu.RLock()
						newIdx := e.currentIdx
						e.mu.RUnlock()
						// 新引擎也触发识别
						if newBe, ok := e.engines[newIdx].(BatchEngine); ok {
							newBe.TriggerRecognize()
							// 继续轮询新引擎
							currentEngine = e.engines[newIdx]
							currentIdx = newIdx
							continue
						}
						return e.engines[newIdx].GetResult()
					}
					break
				}
			}

			time.Sleep(50 * time.Millisecond)
		}

		// 超时：使用 sherpa 的结果兜底
		logger.Warnf("[asr] 批处理引擎 %s 超时，使用 sherpa 结果兜底", e.engineType[currentIdx])
	}

	return e.engines[endpointIdx].GetResult()
}

// IsEndpoint 实现 Engine 接口。
// 使用最后一个引擎（通常是 sherpa）来做端点检测，
// 因为在线引擎（如腾讯云一句话识别）不支持实时端点检测。
// 端点触发时，通知批处理引擎准备识别，并设置端点标记。
func (e *FallbackEngine) IsEndpoint() bool {
	isEndpoint := e.engines[e.endpointDetectorIdx].IsEndpoint()
	if isEndpoint {
		// 设置端点触发标记
		e.mu.Lock()
		e.endpointTriggered = true
		e.mu.Unlock()

		// 通知所有批处理引擎：端点已触发，启动异步识别
		for _, engine := range e.engines {
			if be, ok := engine.(BatchEngine); ok {
				be.TriggerRecognize()
			}
		}
	}
	return isEndpoint
}

// Reset 实现 Engine 接口。
func (e *FallbackEngine) Reset() {
	e.mu.Lock()
	e.endpointTriggered = false
	e.mu.Unlock()
	// 重置所有引擎
	for _, engine := range e.engines {
		engine.Reset()
	}
}

// Cancel 取消正在进行的识别。
func (e *FallbackEngine) Cancel() {
	// 取消所有支持取消的引擎
	for _, engine := range e.engines {
		if c, ok := engine.(interface{ Cancel() }); ok {
			c.Cancel()
		}
	}
}

// Close 实现 Engine 接口。
func (e *FallbackEngine) Close() {
	for _, engine := range e.engines {
		engine.Close()
	}
	logger.Info("[asr] Fallback 引擎已关闭")
}

// Name 实现 Engine 接口。
func (e *FallbackEngine) Name() string {
	return string(e.engineType[e.currentIdx])
}

// CurrentType 返回当前引擎类型。
func (e *FallbackEngine) CurrentType() EngineType {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.engineType[e.currentIdx]
}

// IsDegraded 返回是否处于降级状态（使用非首选引擎）。
func (e *FallbackEngine) IsDegraded() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.currentIdx > 0
}

// IsQuotaExhaustedError 判断是否为额度耗尽错误。
func IsQuotaExhaustedError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()

	// 腾讯云额度耗尽相关错误码
	quotaErrors := []string{
		"ResourceInsufficient",     // 资源不足
		"QuotaExhausted",           // 额度耗尽
		"InvalidParameter.Resource", // 资源不存在（可能免费额度用完）
	}

	for _, code := range quotaErrors {
		if strings.Contains(errStr, code) {
			return true
		}
	}
	return false
}

// IsNetworkError 判断是否为网络错误。
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())

	networkErrors := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"no such host",
		"network is unreachable",
		"i/o timeout",
		"eof",
	}

	for _, pattern := range networkErrors {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}
	return false
}

package pipeline

import (
	"log"
	"sync"
)

// State 表示流水线的当前运行状态。
type State int

const (
	// StateIdle — 空闲，等待唤醒词。
	StateIdle State = iota
	// StateListening — 正在录音（VAD + ASR 活跃）。
	StateListening
	// StateProcessing — 正在将 ASR 结果发送给 LLM。
	StateProcessing
	// StateSpeaking — 正在播放 TTS 音频。
	StateSpeaking
)

var stateNames = [...]string{
	"Idle",
	"Listening",
	"Processing",
	"Speaking",
}

func (s State) String() string {
	if int(s) < len(stateNames) {
		return stateNames[s]
	}
	return "Unknown"
}

// StateMachine 管理线程安全的状态转换。
type StateMachine struct {
	mu       sync.RWMutex
	current  State
	onChange func(from, to State)
}

// NewStateMachine 创建一个初始状态为 Idle 的状态机。
func NewStateMachine() *StateMachine {
	return &StateMachine{
		current: StateIdle,
	}
}

// SetOnChange 注册状态变化时的回调函数。
func (sm *StateMachine) SetOnChange(fn func(from, to State)) {
	sm.mu.Lock()
	sm.onChange = fn
	sm.mu.Unlock()
}

// Current 返回当前状态。
func (sm *StateMachine) Current() State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.current
}

// Transition 尝试切换状态。只有合法的转换才会生效：
//
//	Idle       → Listening   （检测到唤醒词）
//	Listening  → Processing  （语音结束）
//	Processing → Speaking    （TTS 开始播放）
//	Speaking   → Idle        （播放完毕或被打断）
//
// 任何状态都可以转换到 Idle（用于错误恢复或唤醒词打断）。
func (sm *StateMachine) Transition(to State) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if !validTransition(sm.current, to) {
		log.Printf("[state] 非法转换 %s → %s", sm.current, to)
		return false
	}

	from := sm.current
	sm.current = to
	log.Printf("[state] %s → %s", from, to)

	if sm.onChange != nil {
		sm.onChange(from, to)
	}
	return true
}

// ForceIdle 无条件重置状态为 Idle。
func (sm *StateMachine) ForceIdle() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	from := sm.current
	sm.current = StateIdle
	if from != StateIdle {
		log.Printf("[state] 强制重置 %s → Idle", from)
		if sm.onChange != nil {
			sm.onChange(from, StateIdle)
		}
	}
}

// validTransition 检查状态转换是否合法。
func validTransition(from, to State) bool {
	// 始终允许重置到 Idle（用于打断/错误恢复）
	if to == StateIdle {
		return true
	}
	switch from {
	case StateIdle:
		return to == StateListening
	case StateListening:
		return to == StateProcessing
	case StateProcessing:
		return to == StateSpeaking
	case StateSpeaking:
		return to == StateIdle
	}
	return false
}

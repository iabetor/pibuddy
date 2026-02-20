package voiceprint

import (
	"fmt"
	"sync"

	"github.com/iabetor/pibuddy/internal/config"
	"github.com/iabetor/pibuddy/internal/logger"
	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// Manager 是声纹识别的编排层，统一入口。
type Manager struct {
	extractor *Extractor
	store     *Store
	spkMgr    *sherpa.SpeakerEmbeddingManager
	threshold float32
	mu        sync.RWMutex
}

// NewManager 创建声纹识别管理器。
// 加载模型 → 打开 SQLite → 创建内存搜索索引 → 从 DB 加载已注册用户。
func NewManager(cfg config.VoiceprintConfig, dataDir string) (*Manager, error) {
	extractor, err := NewExtractor(cfg.ModelPath, cfg.NumThreads)
	if err != nil {
		return nil, fmt.Errorf("创建声纹提取器失败: %w", err)
	}

	store, err := NewStore(dataDir)
	if err != nil {
		extractor.Close()
		return nil, fmt.Errorf("创建声纹存储失败: %w", err)
	}

	spkMgr := sherpa.NewSpeakerEmbeddingManager(extractor.Dim())
	if spkMgr == nil {
		store.Close()
		extractor.Close()
		return nil, fmt.Errorf("创建 SpeakerEmbeddingManager 失败")
	}

	m := &Manager{
		extractor: extractor,
		store:     store,
		spkMgr:    spkMgr,
		threshold: cfg.Threshold,
	}

	// 从 DB 加载已注册用户到内存索引
	if err := m.loadFromDB(); err != nil {
		m.Close()
		return nil, fmt.Errorf("加载声纹数据失败: %w", err)
	}

	logger.Infof("[voiceprint] 声纹管理器已初始化 (speakers=%d, threshold=%.2f)", m.spkMgr.NumSpeakers(), cfg.Threshold)

	return m, nil
}

// loadFromDB 从数据库加载所有 embedding 到内存索引。
func (m *Manager) loadFromDB() error {
	allEmbeddings, err := m.store.GetAllEmbeddings()
	if err != nil {
		return err
	}

	// 按用户名分组
	grouped := make(map[string][][]float32)
	for _, ue := range allEmbeddings {
		grouped[ue.UserName] = append(grouped[ue.UserName], ue.Embedding)
	}

	// 注册到内存索引
	for name, embeddings := range grouped {
		if !m.spkMgr.RegisterV(name, embeddings) {
			logger.Warnf("[voiceprint] 警告: 注册用户 %s 到内存索引失败", name)
		}
	}

	return nil
}

// Identify 识别说话人。返回用户名，未识别时返回空字符串。
func (m *Manager) Identify(samples []float32) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.spkMgr.NumSpeakers() == 0 {
		return "", nil
	}

	embedding, err := m.extractor.Extract(samples)
	if err != nil {
		return "", fmt.Errorf("提取声纹失败: %w", err)
	}

	name := m.spkMgr.Search(embedding, m.threshold)
	if name != "" {
		logger.Infof("[voiceprint] 识别到用户: %s (阈值: %.2f)", name, m.threshold)
	} else {
		// 尝试用最低阈值搜索，看看最接近谁（用于调试）
		bestName := m.spkMgr.Search(embedding, 0.01)
		if bestName != "" {
			// 使用 Verify 在不同阈值下检测，粗略估算分数
			score := m.estimateScore(bestName, embedding)
			logger.Infof("[voiceprint] 未达阈值，最接近: %s (估算置信度: ~%.2f, 阈值: %.2f)", bestName, score, m.threshold)
		} else {
			logger.Infof("[voiceprint] 未识别到任何用户 (阈值: %.2f)", m.threshold)
		}
	}
	return name, nil
}

// estimateScore 通过二分法 Verify 粗略估算匹配分数（sherpa API 不直接暴露分数）。
func (m *Manager) estimateScore(name string, embedding []float32) float32 {
	low, high := float32(0.0), float32(1.0)
	for i := 0; i < 10; i++ {
		mid := (low + high) / 2
		if m.spkMgr.Verify(name, embedding, mid) {
			low = mid
		} else {
			high = mid
		}
	}
	return (low + high) / 2
}

// Register 注册新用户。audioSamples 是多个音频样本，每个至少 1 秒。
func (m *Manager) Register(name string, audioSamples [][]float32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 提取所有 embedding
	var embeddings [][]float32
	for i, samples := range audioSamples {
		embedding, err := m.extractor.Extract(samples)
		if err != nil {
			return fmt.Errorf("提取第 %d 个样本的声纹失败: %w", i+1, err)
		}
		embeddings = append(embeddings, embedding)
	}

	// 存入 DB
	userID, err := m.store.AddUser(name)
	if err != nil {
		return fmt.Errorf("添加用户失败: %w", err)
	}

	for _, emb := range embeddings {
		if err := m.store.AddEmbedding(userID, emb); err != nil {
			return fmt.Errorf("存储 embedding 失败: %w", err)
		}
	}

	// 先移除旧的（如果存在），再重新注册到内存索引
	if m.spkMgr.Contains(name) {
		m.spkMgr.Remove(name)
	}

	// 获取该用户所有 embedding（包括之前的）
	allEmbeddings, err := m.store.GetAllEmbeddings()
	if err != nil {
		return fmt.Errorf("获取用户 embeddings 失败: %w", err)
	}

	var userEmbeddings [][]float32
	for _, ue := range allEmbeddings {
		if ue.UserName == name {
			userEmbeddings = append(userEmbeddings, ue.Embedding)
		}
	}

	if !m.spkMgr.RegisterV(name, userEmbeddings) {
		return fmt.Errorf("注册用户 %s 到内存索引失败", name)
	}

	logger.Infof("[voiceprint] 用户 %s 注册成功 (%d 个样本)", name, len(audioSamples))
	return nil
}

// ListUsers 列出所有已注册的声纹用户。
func (m *Manager) ListUsers() ([]User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.store.ListUsers()
}

// DeleteUser 删除用户及其所有声纹数据。
func (m *Manager) DeleteUser(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.store.DeleteUser(name); err != nil {
		return err
	}

	m.spkMgr.Remove(name)
	logger.Infof("[voiceprint] 用户 %s 已删除", name)
	return nil
}

// NumSpeakers 返回已注册的说话人数量。
func (m *Manager) NumSpeakers() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.spkMgr.NumSpeakers()
}

// SetOwner 设置用户为主人。
func (m *Manager) SetOwner(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.store.SetOwner(name)
}

// GetOwner 获取主人信息。
func (m *Manager) GetOwner() (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.store.GetOwner()
}

// IsOwner 检查指定用户是否是主人。
func (m *Manager) IsOwner(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, err := m.store.GetUser(name)
	if err != nil || user == nil {
		return false
	}
	return user.IsOwner()
}

// SetPreferences 设置用户偏好。
func (m *Manager) SetPreferences(name string, preferences string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.store.SetPreferences(name, preferences)
}

// GetUser 获取用户信息（包含偏好）。
func (m *Manager) GetUser(name string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.store.GetUser(name)
}

// Close 释放所有资源。
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.spkMgr != nil {
		sherpa.DeleteSpeakerEmbeddingManager(m.spkMgr)
		m.spkMgr = nil
	}
	if m.store != nil {
		m.store.Close()
	}
	if m.extractor != nil {
		m.extractor.Close()
	}

	logger.Info("[voiceprint] 声纹管理器已关闭")
}

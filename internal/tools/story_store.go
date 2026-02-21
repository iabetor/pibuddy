package tools

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/iabetor/pibuddy/internal/database"
	"github.com/iabetor/pibuddy/internal/logger"
)

// Story 表示一个故事
type Story struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Category   string    `json:"category"`
	Tags       []string  `json:"tags"`
	Content    string    `json:"content"`
	WordCount  int       `json:"word_count"`
	Source     string    `json:"source"` // local/api/llm
	PlayCount  int       `json:"play_count"`
	Rating     float64   `json:"rating"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// StoryStore 故事存储（SQLite）
type StoryStore struct {
	db *database.DB
}

// NewStoryStore 创建故事存储
func NewStoryStore(db *database.DB) (*StoryStore, error) {
	s := &StoryStore{
		db: db,
	}

	count := s.Count()
	logger.Infof("[story] 故事库已加载，共 %d 个故事", count)
	return s, nil
}

// FindStory 查找故事
func (s *StoryStore) FindStory(keyword string) (*Story, error) {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	
	var query string
	var args []interface{}
	
	if keyword == "" {
		// 随机返回一个故事
		query = `SELECT id, title, category, tags, content, word_count, source, play_count, rating, created_at, updated_at 
		         FROM stories ORDER BY RANDOM() LIMIT 1`
		args = nil
	} else {
		// 按优先级查找：标题精确匹配 > 标题模糊匹配 > 标签匹配 > 分类匹配
		query = `SELECT id, title, category, tags, content, word_count, source, play_count, rating, created_at, updated_at 
		         FROM stories 
		         WHERE LOWER(title) = ? 
		            OR LOWER(title) LIKE ? 
		            OR LOWER(tags) LIKE ? 
		            OR LOWER(category) LIKE ?
		         ORDER BY 
		            CASE WHEN LOWER(title) = ? THEN 0 
		                 WHEN LOWER(title) LIKE ? THEN 1 
		                 ELSE 2 END
		         LIMIT 1`
		searchPattern := "%" + keyword + "%"
		args = []interface{}{keyword, searchPattern, searchPattern, searchPattern, keyword, searchPattern}
	}

	var story Story
	var tagsJSON string
	var createdAt, updatedAt sql.NullTime

	err := s.db.QueryRow(query, args...).Scan(
		&story.ID, &story.Title, &story.Category, &tagsJSON, &story.Content,
		&story.WordCount, &story.Source, &story.PlayCount, &story.Rating,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询故事失败: %w", err)
	}

	// 解析 tags
	if tagsJSON != "" {
		json.Unmarshal([]byte(tagsJSON), &story.Tags)
	}
	if createdAt.Valid {
		story.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		story.UpdatedAt = updatedAt.Time
	}

	// 更新播放计数
	go s.incrementPlayCount(story.ID)

	return &story, nil
}

// GetByID 根据 ID 获取故事
func (s *StoryStore) GetByID(id string) (*Story, error) {
	query := `SELECT id, title, category, tags, content, word_count, source, play_count, rating, created_at, updated_at 
	          FROM stories WHERE id = ?`

	var story Story
	var tagsJSON string
	var createdAt, updatedAt sql.NullTime

	err := s.db.QueryRow(query, id).Scan(
		&story.ID, &story.Title, &story.Category, &tagsJSON, &story.Content,
		&story.WordCount, &story.Source, &story.PlayCount, &story.Rating,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询故事失败: %w", err)
	}

	if tagsJSON != "" {
		json.Unmarshal([]byte(tagsJSON), &story.Tags)
	}
	if createdAt.Valid {
		story.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		story.UpdatedAt = updatedAt.Time
	}

	return &story, nil
}

// SaveStory 保存故事（用于新增或更新）
func (s *StoryStore) SaveStory(story *Story) error {
	if story.ID == "" {
		story.ID = generateStoryID(story.Title)
	}
	if story.Source == "" {
		story.Source = "local"
	}
	story.WordCount = len([]rune(story.Content))
	story.UpdatedAt = time.Now()
	if story.CreatedAt.IsZero() {
		story.CreatedAt = story.UpdatedAt
	}

	tagsJSON, _ := json.Marshal(story.Tags)

	query := `INSERT OR REPLACE INTO stories 
	          (id, title, category, tags, content, word_count, source, play_count, rating, created_at, updated_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.Exec(query,
		story.ID, story.Title, story.Category, string(tagsJSON), story.Content,
		story.WordCount, story.Source, story.PlayCount, story.Rating,
		story.CreatedAt, story.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("保存故事失败: %w", err)
	}

	logger.Debugf("[story] 已保存故事: %s (来源: %s)", story.Title, story.Source)
	return nil
}

// incrementPlayCount 增加播放计数
func (s *StoryStore) incrementPlayCount(id string) {
	s.db.Exec("UPDATE stories SET play_count = play_count + 1 WHERE id = ?", id)
}

// ListCategories 列出所有分类
func (s *StoryStore) ListCategories() []string {
	query := `SELECT DISTINCT category FROM stories ORDER BY category`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var cat string
		if err := rows.Scan(&cat); err != nil {
			continue
		}
		categories = append(categories, cat)
	}
	return categories
}

// ListStories 列出故事
func (s *StoryStore) ListStories(category string, limit int) []Story {
	var query string
	var args []interface{}

	if limit <= 0 {
		limit = 10
	}

	if category == "" {
		query = `SELECT id, title, category, tags, content, word_count, source, play_count, rating, created_at, updated_at 
		         FROM stories ORDER BY play_count DESC LIMIT ?`
		args = []interface{}{limit}
	} else {
		query = `SELECT id, title, category, tags, content, word_count, source, play_count, rating, created_at, updated_at 
		         FROM stories WHERE category = ? ORDER BY play_count DESC LIMIT ?`
		args = []interface{}{category, limit}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var stories []Story
	for rows.Next() {
		var story Story
		var tagsJSON string
		var createdAt, updatedAt sql.NullTime

		if err := rows.Scan(
			&story.ID, &story.Title, &story.Category, &tagsJSON, &story.Content,
			&story.WordCount, &story.Source, &story.PlayCount, &story.Rating,
			&createdAt, &updatedAt,
		); err != nil {
			continue
		}

		if tagsJSON != "" {
			json.Unmarshal([]byte(tagsJSON), &story.Tags)
		}
		if createdAt.Valid {
			story.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			story.UpdatedAt = updatedAt.Time
		}
		stories = append(stories, story)
	}

	return stories
}

// Count 返回故事总数
func (s *StoryStore) Count() int {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM stories").Scan(&count)
	return count
}

// CountBySource 按来源统计故事数量
func (s *StoryStore) CountBySource(source string) int {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM stories WHERE source = ?", source).Scan(&count)
	return count
}

// DeleteStory 删除故事（只能删除 source=api 或 source=llm 的故事）
func (s *StoryStore) DeleteStory(id string) error {
	// 先检查故事来源
	var source string
	err := s.db.QueryRow("SELECT source FROM stories WHERE id = ?", id).Scan(&source)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("故事不存在")
		}
		return fmt.Errorf("查询故事失败: %w", err)
	}

	// 禁止删除内置故事
	if source == "local" {
		return fmt.Errorf("不能删除内置故事")
	}

	// 执行删除
	result, err := s.db.Exec("DELETE FROM stories WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("删除故事失败: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("故事不存在")
	}

	logger.Debugf("[story] 已删除故事: %s", id)
	return nil
}

// DeleteByKeyword 根据关键词删除故事（只能删除 source=api 或 source=llm 的故事）
func (s *StoryStore) DeleteByKeyword(keyword string) (int, error) {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return 0, fmt.Errorf("关键词不能为空")
	}

	// 查找匹配的故事
	searchPattern := "%" + keyword + "%"
	query := `SELECT id, title, source FROM stories 
	          WHERE (LOWER(title) = ? OR LOWER(title) LIKE ?) 
	            AND source != 'local'`

	rows, err := s.db.Query(query, keyword, searchPattern)
	if err != nil {
		return 0, fmt.Errorf("查询故事失败: %w", err)
	}
	defer rows.Close()

	var ids []string
	var titles []string
	for rows.Next() {
		var id, title, source string
		if err := rows.Scan(&id, &title, &source); err != nil {
			continue
		}
		ids = append(ids, id)
		titles = append(titles, title)
	}

	if len(ids) == 0 {
		return 0, nil
	}

	// 删除找到的故事
	for i, id := range ids {
		_, err := s.db.Exec("DELETE FROM stories WHERE id = ?", id)
		if err != nil {
			logger.Warnf("[story] 删除故事《%s》失败: %v", titles[i], err)
		} else {
			logger.Debugf("[story] 已删除故事: %s", titles[i])
		}
	}

	return len(ids), nil
}

// DeleteAllUserStories 删除所有用户保存的故事（source=llm）
func (s *StoryStore) DeleteAllUserStories() (int, error) {
	result, err := s.db.Exec("DELETE FROM stories WHERE source = ?", "llm")
	if err != nil {
		return 0, fmt.Errorf("删除故事失败: %w", err)
	}

	affected, _ := result.RowsAffected()
	logger.Debugf("[story] 已删除 %d 个用户保存的故事", affected)
	return int(affected), nil
}

// DeleteAllCachedStories 删除所有 API 缓存的故事（source=api）
func (s *StoryStore) DeleteAllCachedStories() (int, error) {
	result, err := s.db.Exec("DELETE FROM stories WHERE source = ?", "api")
	if err != nil {
		return 0, fmt.Errorf("删除故事失败: %w", err)
	}

	affected, _ := result.RowsAffected()
	logger.Debugf("[story] 已删除 %d 个 API 缓存的故事", affected)
	return int(affected), nil
}

// generateStoryID 根据标题生成故事 ID
func generateStoryID(title string) string {
	// 使用时间戳 + 标题哈希
	return fmt.Sprintf("%d_%x", time.Now().Unix(), hashString(title)[:8])
}

func hashString(s string) string {
	// 简单哈希
	h := uint32(0)
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return fmt.Sprintf("%08x", h)
}

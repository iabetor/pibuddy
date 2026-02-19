package voiceprint

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"github.com/iabetor/pibuddy/internal/logger"
	"math"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// User 表示一个已注册的声纹用户。
type User struct {
	ID          int64
	Name        string
	isOwner     bool    // 私有字段，避免与方法冲突
	Preferences string // JSON 格式的用户偏好
}

// GetPreferences 实现 UserPreferences 接口。
func (u *User) GetPreferences() string {
	return u.Preferences
}

// IsOwner 实现 UserPreferences 接口。
func (u *User) IsOwner() bool {
	return u.isOwner
}

// UserPreferences 用户偏好结构。
type UserPreferences struct {
	Style      string   `json:"style,omitempty"`      // 回复风格，如"简洁直接"
	Interests  []string `json:"interests,omitempty"`  // 兴趣爱好
	Nickname   string   `json:"nickname,omitempty"`   // 昵称
	Extra      string   `json:"extra,omitempty"`      // 额外描述
}

// UserEmbedding 表示用户的一条 embedding 记录。
type UserEmbedding struct {
	UserName  string
	Embedding []float32
}

// Store 使用 SQLite 持久化声纹数据。
type Store struct {
	db *sql.DB
}

// NewStore 创建声纹存储。
// dataDir: 数据目录路径，SQLite 文件存放在此目录下。
func NewStore(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	dbPath := filepath.Join(dataDir, "voiceprint.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 设置 WAL 模式和外键约束
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("设置 WAL 模式失败: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("启用外键约束失败: %w", err)
	}

	// 创建表
	if err := createTables(db); err != nil {
		db.Close()
		return nil, err
	}

	logger.Infof("[voiceprint] 声纹存储已初始化 (db=%s)", dbPath)

	return &Store{db: db}, nil
}

func createTables(db *sql.DB) error {
	// 先创建基础表（如果不存在）
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS embeddings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			embedding BLOB NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return fmt.Errorf("创建数据表失败: %w", err)
	}

	// 添加新字段（如果不存在）
	migrations := []string{
		"ALTER TABLE users ADD COLUMN is_owner BOOLEAN DEFAULT 0",
		"ALTER TABLE users ADD COLUMN preferences TEXT DEFAULT ''",
	}
	for _, m := range migrations {
		// SQLite 不支持 IF NOT EXISTS for ALTER TABLE，忽略错误
		db.Exec(m)
	}

	return nil
}

// AddUser 添加用户，返回用户 ID。如果用户已存在则返回已有 ID。
func (s *Store) AddUser(name string) (int64, error) {
	result, err := s.db.Exec("INSERT OR IGNORE INTO users (name) VALUES (?)", name)
	if err != nil {
		return 0, fmt.Errorf("添加用户失败: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		id, err := result.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("获取用户 ID 失败: %w", err)
		}
		return id, nil
	}

	// 用户已存在，查询 ID
	var id int64
	err = s.db.QueryRow("SELECT id FROM users WHERE name = ?", name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("查询用户 ID 失败: %w", err)
	}
	return id, nil
}

// AddEmbedding 添加一条 embedding 记录。
func (s *Store) AddEmbedding(userID int64, embedding []float32) error {
	blob := float32ToBytes(embedding)
	_, err := s.db.Exec("INSERT INTO embeddings (user_id, embedding) VALUES (?, ?)", userID, blob)
	if err != nil {
		return fmt.Errorf("添加 embedding 失败: %w", err)
	}
	return nil
}

// GetUser 根据名称获取用户。
func (s *Store) GetUser(name string) (*User, error) {
	var u User
	err := s.db.QueryRow("SELECT id, name, is_owner, preferences FROM users WHERE name = ?", name).Scan(&u.ID, &u.Name, &u.isOwner, &u.Preferences)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	return &u, nil
}

// ListUsers 列出所有用户。
func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query("SELECT id, name, is_owner, preferences FROM users ORDER BY is_owner DESC, id")
	if err != nil {
		return nil, fmt.Errorf("列出用户失败: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.isOwner, &u.Preferences); err != nil {
			return nil, fmt.Errorf("读取用户数据失败: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// DeleteUser 删除用户及其所有 embedding（级联删除）。
func (s *Store) DeleteUser(name string) error {
	result, err := s.db.Exec("DELETE FROM users WHERE name = ?", name)
	if err != nil {
		return fmt.Errorf("删除用户失败: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("用户 %s 不存在", name)
	}
	return nil
}

// SetOwner 设置用户为主人。ownerName 只能有一个主人，设置新主人会取消旧主人。
func (s *Store) SetOwner(name string) error {
	// 先取消所有主人
	if _, err := s.db.Exec("UPDATE users SET is_owner = 0"); err != nil {
		return fmt.Errorf("取消旧主人失败: %w", err)
	}
	// 设置新主人
	result, err := s.db.Exec("UPDATE users SET is_owner = 1 WHERE name = ?", name)
	if err != nil {
		return fmt.Errorf("设置主人失败: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("用户 %s 不存在", name)
	}
	return nil
}

// GetOwner 获取主人信息。如果没有主人返回 nil。
func (s *Store) GetOwner() (*User, error) {
	var u User
	err := s.db.QueryRow("SELECT id, name, is_owner, preferences FROM users WHERE is_owner = 1").Scan(&u.ID, &u.Name, &u.isOwner, &u.Preferences)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询主人失败: %w", err)
	}
	return &u, nil
}

// SetPreferences 设置用户偏好。
func (s *Store) SetPreferences(name string, preferences string) error {
	result, err := s.db.Exec("UPDATE users SET preferences = ? WHERE name = ?", preferences, name)
	if err != nil {
		return fmt.Errorf("设置偏好失败: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("用户 %s 不存在", name)
	}
	return nil
}

// GetAllEmbeddings 获取所有用户的 embedding，用于启动时加载到内存索引。
func (s *Store) GetAllEmbeddings() ([]UserEmbedding, error) {
	rows, err := s.db.Query(`
		SELECT u.name, e.embedding
		FROM embeddings e
		JOIN users u ON u.id = e.user_id
		ORDER BY u.name, e.id
	`)
	if err != nil {
		return nil, fmt.Errorf("获取 embeddings 失败: %w", err)
	}
	defer rows.Close()

	var result []UserEmbedding
	for rows.Next() {
		var ue UserEmbedding
		var blob []byte
		if err := rows.Scan(&ue.UserName, &blob); err != nil {
			return nil, fmt.Errorf("读取 embedding 数据失败: %w", err)
		}
		ue.Embedding = bytesToFloat32(blob)
		result = append(result, ue)
	}
	return result, rows.Err()
}

// Close 关闭数据库连接。
func (s *Store) Close() {
	if s.db != nil {
		s.db.Close()
		logger.Info("[voiceprint] 声纹存储已关闭")
	}
}

// float32ToBytes 将 []float32 序列化为小端字节序 BLOB。
func float32ToBytes(data []float32) []byte {
	buf := make([]byte, len(data)*4)
	for i, v := range data {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// bytesToFloat32 将小端字节序 BLOB 反序列化为 []float32。
func bytesToFloat32(data []byte) []float32 {
	result := make([]float32, len(data)/4)
	for i := range result {
		result[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return result
}

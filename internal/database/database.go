package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/iabetor/pibuddy/internal/logger"
	_ "modernc.org/sqlite"
)

// DB 是统一的 SQLite 数据库连接。
// 所有模块共享同一个数据库文件，便于事务和备份。
type DB struct {
	*sql.DB
	path string
}

// Open 打开或创建数据库。
// dbPath: 数据库文件路径，如果为空则使用默认路径 ~/.pibuddy/pibuddy.db
func Open(dbPath string) (*DB, error) {
	if dbPath == "" {
		home, _ := os.UserHomeDir()
		if home != "" {
			dbPath = filepath.Join(home, ".pibuddy", "pibuddy.db")
		} else {
			dbPath = "./pibuddy.db"
		}
	}

	// 确保目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 设置 WAL 模式（更好的并发性能）
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("设置 WAL 模式失败: %w", err)
	}

	// 启用外键约束
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("启用外键约束失败: %w", err)
	}

	logger.Infof("[database] 数据库已打开: %s", dbPath)

	return &DB{DB: db, path: dbPath}, nil
}

// Path 返回数据库文件路径。
func (db *DB) Path() string {
	return db.path
}

// Migrate 运行数据库迁移。
func (db *DB) Migrate() error {
	// 创建所有模块的表
	migrations := []string{
		// 声纹用户表
		`CREATE TABLE IF NOT EXISTS voiceprint_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			is_owner BOOLEAN DEFAULT 0,
			preferences TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// 声纹 embedding 表
		`CREATE TABLE IF NOT EXISTS voiceprint_embeddings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES voiceprint_users(id) ON DELETE CASCADE,
			embedding BLOB NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// 音乐缓存索引表
		`CREATE TABLE IF NOT EXISTS music_cache (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			cache_key TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			artist TEXT DEFAULT '',
			album TEXT DEFAULT '',
			provider TEXT NOT NULL,
			provider_id INTEGER NOT NULL,
			duration INTEGER DEFAULT 0,
			size INTEGER DEFAULT 0,
			play_count INTEGER DEFAULT 0,
			cached_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_played DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// 音乐收藏表
		`CREATE TABLE IF NOT EXISTS music_favorites (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			artist TEXT DEFAULT '',
			album TEXT DEFAULT '',
			provider TEXT NOT NULL,
			provider_id INTEGER NOT NULL,
			song_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(provider, provider_id)
		)`,
		// ASR 使用统计表
		`CREATE TABLE IF NOT EXISTS asr_stats (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			engine TEXT NOT NULL,
			date TEXT NOT NULL,
			count INTEGER DEFAULT 0,
			UNIQUE(engine, date)
		)`,
		// 系统配置表
		`CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// 故事表
		`CREATE TABLE IF NOT EXISTS stories (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			category TEXT NOT NULL,
			tags TEXT DEFAULT '[]',
			content TEXT NOT NULL,
			word_count INTEGER DEFAULT 0,
			source TEXT DEFAULT 'local',
			play_count INTEGER DEFAULT 0,
			rating REAL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("数据库迁移失败: %w", err)
		}
	}

	// 创建索引
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_music_cache_name ON music_cache(name)`,
		`CREATE INDEX IF NOT EXISTS idx_music_cache_artist ON music_cache(artist)`,
		`CREATE INDEX IF NOT EXISTS idx_music_cache_last_played ON music_cache(last_played)`,
		`CREATE INDEX IF NOT EXISTS idx_music_favorites_name ON music_favorites(name)`,
	}

	for _, idx := range indexes {
		if _, err := db.Exec(idx); err != nil {
			logger.Warnf("[database] 创建索引失败: %v", err)
		}
	}

	// 创建故事表索引
	storyIndexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_stories_title ON stories(title)`,
		`CREATE INDEX IF NOT EXISTS idx_stories_category ON stories(category)`,
		`CREATE INDEX IF NOT EXISTS idx_stories_source ON stories(source)`,
	}
	for _, idx := range storyIndexes {
		if _, err := db.Exec(idx); err != nil {
			logger.Warnf("[database] 创建故事索引失败: %v", err)
		}
	}

	logger.Info("[database] 数据库迁移完成")
	return nil
}

// InitStories 初始化内置故事数据。
// sqlPath: SQL 初始化脚本路径，如果为空则使用默认路径。
func (db *DB) InitStories(sqlPath string) error {
	if sqlPath == "" {
		// 默认路径
		sqlPath = "./scripts/migrations/002_stories_init.sql"
	}

	// 检查文件是否存在
	if _, err := os.Stat(sqlPath); os.IsNotExist(err) {
		logger.Debugf("[database] 故事初始化脚本不存在: %s", sqlPath)
		return nil
	}

	// 读取 SQL 文件
	sqlContent, err := os.ReadFile(sqlPath)
	if err != nil {
		return fmt.Errorf("读取故事初始化脚本失败: %w", err)
	}

	// 执行 SQL
	if _, err := db.Exec(string(sqlContent)); err != nil {
		return fmt.Errorf("执行故事初始化脚本失败: %w", err)
	}

	// 统计导入的故事数量
	var count int
	db.QueryRow("SELECT COUNT(*) FROM stories WHERE source = 'local'").Scan(&count)
	logger.Infof("[database] 已初始化 %d 个内置故事", count)

	return nil
}

// Close 关闭数据库连接。
func (db *DB) Close() error {
	if db.DB != nil {
		return db.DB.Close()
	}
	return nil
}

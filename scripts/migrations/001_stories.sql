-- 故事表迁移脚本
-- 用于手动创建 stories 表（通常由程序自动执行）

-- 故事表
CREATE TABLE IF NOT EXISTS stories (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    category TEXT NOT NULL,
    tags TEXT DEFAULT '[]',
    content TEXT NOT NULL,
    word_count INTEGER DEFAULT 0,
    source TEXT DEFAULT 'local',  -- local/api/llm
    play_count INTEGER DEFAULT 0,
    rating REAL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_stories_title ON stories(title);
CREATE INDEX IF NOT EXISTS idx_stories_category ON stories(category);
CREATE INDEX IF NOT EXISTS idx_stories_source ON stories(source);

-- 全文搜索索引（可选，用于更复杂的搜索）
-- CREATE VIRTUAL TABLE IF NOT EXISTS stories_fts USING fts5(title, content, content='stories', content_rowid=rowid);

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// EnglishWordTool 单词查询工具（有道词典）。
type EnglishWordTool struct {
	client *http.Client
}

// NewEnglishWordTool 创建单词查询工具。
func NewEnglishWordTool() *EnglishWordTool {
	return &EnglishWordTool{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Name 返回工具名称。
func (t *EnglishWordTool) Name() string {
	return "english_word"
}

// Description 返回工具描述。
func (t *EnglishWordTool) Description() string {
	return `查询单词的释义和发音。返回单词的中文意思、音标和发音链接。
例如：查询 "hello" 返回 "你好，音标 /həˈləʊ/"`
}

// Parameters 返回工具参数定义。
func (t *EnglishWordTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"word": {
				"type": "string",
				"description": "要查询的英文单词"
			}
		},
		"required": ["word"]
	}`)
}

// Execute 执行工具。
func (t *EnglishWordTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Word string `json:"word"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	if params.Word == "" {
		return "", fmt.Errorf("请提供要查询的单词")
	}

	return t.queryWord(params.Word)
}

// queryWord 查询单词。
func (t *EnglishWordTool) queryWord(word string) (string, error) {
	// 使用有道词典 suggest API
	apiURL := fmt.Sprintf("http://dict.youdao.com/suggest?doctype=json&q=%s", url.QueryEscape(word))

	resp, err := t.client.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("查询失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析响应
	var result struct {
		Data struct {
			Entries []struct {
				Entry string `json:"entry"`
				Explain string `json:"explain"`
			} `json:"entries"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if len(result.Data.Entries) == 0 || result.Data.Entries[0].Explain == "" {
		return "", fmt.Errorf("未找到单词 %q 的释义", word)
	}

	entry := result.Data.Entries[0]
	
	// 获取发音 URL
	ukAudio := fmt.Sprintf("http://dict.youdao.com/dictvoice?audio=%s&type=1", url.QueryEscape(word))
	usAudio := fmt.Sprintf("http://dict.youdao.com/dictvoice?audio=%s&type=2", url.QueryEscape(word))

	return fmt.Sprintf("%s：%s\n英式发音：%s\n美式发音：%s", 
		entry.Entry, entry.Explain, ukAudio, usAudio), nil
}

// EnglishDailyTool 每日一句工具（金山词霸）。
type EnglishDailyTool struct {
	client *http.Client
}

// NewEnglishDailyTool 创建每日一句工具。
func NewEnglishDailyTool() *EnglishDailyTool {
	return &EnglishDailyTool{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Name 返回工具名称。
func (t *EnglishDailyTool) Name() string {
	return "english_daily"
}

// Description 返回工具描述。
func (t *EnglishDailyTool) Description() string {
	return `获取每日一句英语学习内容。返回一句励志或实用的英语句子及其翻译。`
}

// Parameters 返回工具参数定义。
func (t *EnglishDailyTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
}

// Execute 执行工具。
func (t *EnglishDailyTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	return t.getDailyQuote()
}

// getDailyQuote 获取每日一句。
func (t *EnglishDailyTool) getDailyQuote() (string, error) {
	// 金山词霸每日一句 API
	apiURL := "http://open.iciba.com/dsapi/"

	resp, err := t.client.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("获取每日一句失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	var result struct {
		Content string `json:"content"` // 英文
		Note    string `json:"note"`    // 中文
		Picture string `json:"picture"` // 图片
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Content == "" {
		return "", fmt.Errorf("获取每日一句失败，请稍后重试")
	}

	return fmt.Sprintf("%s\n%s", result.Content, result.Note), nil
}

// VocabularyTool 生词本工具。
type VocabularyTool struct {
	store *VocabularyStore
}

// NewVocabularyTool 创建生词本工具。
func NewVocabularyTool(dataDir string) *VocabularyTool {
	return &VocabularyTool{
		store: NewVocabularyStore(dataDir),
	}
}

// Name 返回工具名称。
func (t *VocabularyTool) Name() string {
	return "vocabulary"
}

// Description 返回工具描述。
func (t *VocabularyTool) Description() string {
	return `管理个人生词本。支持添加、列出和删除生词。
操作：
- add: 添加生词
- list: 列出所有生词
- remove: 删除生词`
}

// Parameters 返回工具参数定义。
func (t *VocabularyTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["add", "list", "remove"],
				"description": "操作类型"
			},
			"word": {
				"type": "string",
				"description": "单词（add/remove 时必需）"
			},
			"meaning": {
				"type": "string",
				"description": "释义（add 时可选，不填则自动查询）"
			}
		},
		"required": ["action"]
	}`)
}

// Execute 执行工具。
func (t *VocabularyTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Action  string `json:"action"`
		Word    string `json:"word"`
		Meaning string `json:"meaning"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	switch params.Action {
	case "add":
		if params.Word == "" {
			return "", fmt.Errorf("请提供要添加的单词")
		}
		meaning := params.Meaning
		if meaning == "" {
			// 自动查询单词释义
			wordTool := NewEnglishWordTool()
			result, err := wordTool.queryWord(params.Word)
			if err != nil {
				meaning = "（释义获取失败）"
			} else {
				// 提取释义部分
				parts := strings.SplitN(result, "：", 2)
				if len(parts) > 1 {
					meaning = strings.Split(parts[1], "\n")[0]
				}
			}
		}
		if err := t.store.Add(params.Word, meaning); err != nil {
			return "", err
		}
		return fmt.Sprintf("已将 %q 添加到生词本", params.Word), nil

	case "list":
		words := t.store.List()
		if len(words) == 0 {
			return "生词本是空的", nil
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("生词本共有 %d 个单词：\n", len(words)))
		for i, w := range words {
			sb.WriteString(fmt.Sprintf("%d. %s - %s\n", i+1, w.Word, w.Meaning))
		}
		return sb.String(), nil

	case "remove":
		if params.Word == "" {
			return "", fmt.Errorf("请提供要删除的单词")
		}
		if err := t.store.Remove(params.Word); err != nil {
			return "", err
		}
		return fmt.Sprintf("已从生词本删除 %q", params.Word), nil

	default:
		return "", fmt.Errorf("不支持的操作: %s", params.Action)
	}
}

// VocabularyStore 生词本存储。
type VocabularyStore struct {
	filePath string
}

// VocabularyItem 生词本条目。
type VocabularyItem struct {
	Word    string `json:"word"`
	Meaning string `json:"meaning"`
	AddedAt string `json:"added_at"`
}

// NewVocabularyStore 创建生词本存储。
func NewVocabularyStore(dataDir string) *VocabularyStore {
	return &VocabularyStore{
		filePath: dataDir + "/vocabulary.json",
	}
}

// Add 添加生词。
func (s *VocabularyStore) Add(word, meaning string) error {
	words, err := s.load()
	if err != nil {
		return err
	}

	// 检查是否已存在
	for _, w := range words {
		if strings.EqualFold(w.Word, word) {
			return fmt.Errorf("单词 %q 已在生词本中", word)
		}
	}

	words = append(words, VocabularyItem{
		Word:    word,
		Meaning: meaning,
		AddedAt: time.Now().Format("2006-01-02"),
	})

	return s.save(words)
}

// List 列出生词。
func (s *VocabularyStore) List() []VocabularyItem {
	words, _ := s.load()
	return words
}

// Remove 删除生词。
func (s *VocabularyStore) Remove(word string) error {
	words, err := s.load()
	if err != nil {
		return err
	}

	found := false
	newWords := make([]VocabularyItem, 0, len(words))
	for _, w := range words {
		if !strings.EqualFold(w.Word, word) {
			newWords = append(newWords, w)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("生词本中没有找到 %q", word)
	}

	return s.save(newWords)
}

func (s *VocabularyStore) load() ([]VocabularyItem, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return []VocabularyItem{}, nil // 文件不存在返回空列表
	}

	var result struct {
		Words []VocabularyItem `json:"words"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("解析生词本失败: %w", err)
	}

	return result.Words, nil
}

func (s *VocabularyStore) save(words []VocabularyItem) error {
	result := struct {
		Words []VocabularyItem `json:"words"`
	}{Words: words}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化生词本失败: %w", err)
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	return os.WriteFile(s.filePath, data, 0644)
}

// EnglishQuizTool 单词测验工具。
type EnglishQuizTool struct {
	store   *QuizStore
	session *QuizSession
}

// NewEnglishQuizTool 创建单词测验工具。
func NewEnglishQuizTool(dataDir string) *EnglishQuizTool {
	return &EnglishQuizTool{
		store: NewQuizStore(dataDir),
	}
}

// Name 返回工具名称。
func (t *EnglishQuizTool) Name() string {
	return "english_quiz"
}

// Description 返回工具描述。
func (t *EnglishQuizTool) Description() string {
	return `英语单词测验游戏。系统随机出题，用户回答单词的中文意思。
操作：
- start: 开始测验
- answer: 回答问题
- stop: 结束测验`
}

// Parameters 返回工具参数定义。
func (t *EnglishQuizTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["start", "answer", "stop"],
				"description": "操作类型"
			},
			"answer": {
				"type": "string",
				"description": "答案（answer 时必需）"
			}
		},
		"required": ["action"]
	}`)
}

// Execute 执行工具。
func (t *EnglishQuizTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Action string `json:"action"`
		Answer string `json:"answer"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	switch params.Action {
	case "start":
		return t.startQuiz()
	case "answer":
		return t.answerQuiz(params.Answer)
	case "stop":
		return t.stopQuiz()
	default:
		return "", fmt.Errorf("不支持的操作: %s", params.Action)
	}
}

// QuizSession 测验会话。
type QuizSession struct {
	Word    string
	Meaning string
	Score   int
	Total   int
}

func (t *EnglishQuizTool) startQuiz() (string, error) {
	words := t.store.GetWords()
	if len(words) == 0 {
		return "", fmt.Errorf("词库为空，请先添加生词到生词本")
	}

	// 随机选一个词
	word := words[0] // 简化：选第一个，实际可随机
	t.session = &QuizSession{
		Word:    word.Word,
		Meaning: word.Meaning,
		Score:   0,
		Total:   0,
	}

	return fmt.Sprintf("测验开始！请听题：\n%s 是什么意思？", word.Word), nil
}

func (t *EnglishQuizTool) answerQuiz(answer string) (string, error) {
	if t.session == nil {
		return "", fmt.Errorf("请先开始测验")
	}

	t.session.Total++
	
	// 简单匹配答案
	correct := strings.Contains(strings.ToLower(t.session.Meaning), strings.ToLower(answer))
	
	var result string
	if correct {
		t.session.Score++
		result = fmt.Sprintf("正确！%s 的意思是 %s", t.session.Word, t.session.Meaning)
	} else {
		result = fmt.Sprintf("错误。%s 的意思是 %s", t.session.Word, t.session.Meaning)
	}

	// 出下一题
	words := t.store.GetWords()
	if len(words) > 0 && t.session.Total < 10 {
		word := words[t.session.Total%len(words)]
		t.session.Word = word.Word
		t.session.Meaning = word.Meaning
		result += fmt.Sprintf("\n\n下一题：%s 是什么意思？", word.Word)
	} else {
		result += fmt.Sprintf("\n\n测验结束！得分：%d/%d", t.session.Score, t.session.Total)
		t.session = nil
	}

	return result, nil
}

func (t *EnglishQuizTool) stopQuiz() (string, error) {
	if t.session == nil {
		return "当前没有进行中的测验", nil
	}

	result := fmt.Sprintf("测验结束！得分：%d/%d", t.session.Score, t.session.Total)
	t.session = nil
	return result, nil
}

// QuizStore 测验词库存储。
type QuizStore struct {
	dataDir string
}

// NewQuizStore 创建测验词库存储。
func NewQuizStore(dataDir string) *QuizStore {
	return &QuizStore{dataDir: dataDir}
}

// GetWords 获取词库单词。
func (s *QuizStore) GetWords() []VocabularyItem {
	// 优先使用生词本
	vocabStore := NewVocabularyStore(s.dataDir)
	words := vocabStore.List()
	if len(words) > 0 {
		return words
	}

	// 返回内置词库
	return []VocabularyItem{
		{Word: "abandon", Meaning: "放弃，抛弃"},
		{Word: "absorb", Meaning: "吸收，吸引"},
		{Word: "abstract", Meaning: "抽象的，摘要"},
		{Word: "abundant", Meaning: "丰富的，充裕的"},
		{Word: "academic", Meaning: "学术的，学院的"},
		{Word: "accelerate", Meaning: "加速，促进"},
		{Word: "accept", Meaning: "接受，承认"},
		{Word: "access", Meaning: "访问，通路"},
		{Word: "accident", Meaning: "事故，意外"},
		{Word: "accomplish", Meaning: "完成，实现"},
	}
}

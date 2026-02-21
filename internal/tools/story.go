package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/iabetor/pibuddy/internal/logger"
)

// StoryResult 故事播放结果，供 Pipeline 解析。
type StoryResult struct {
	Success bool   `json:"success"`
	Content string `json:"content,omitempty"` // 故事内容
	Title   string `json:"title,omitempty"`   // 故事标题
	Error   string `json:"error,omitempty"`
	SkipLLM bool   `json:"skip_llm"` // 是否跳过 LLM 直接朗读
}

// TellStoryTool 讲故事工具
type TellStoryTool struct {
	store       *StoryStore
	api         *StoryAPI
	llmFallback bool
	outputMode  string // 输出模式：raw（原文）、summarize（总结）
}

// TellStoryParams 讲故事参数
type TellStoryParams struct {
	Keyword string `json:"keyword"`
}

// NewTellStoryTool 创建讲故事工具
func NewTellStoryTool(store *StoryStore, api *StoryAPI, llmFallback bool, outputMode string) *TellStoryTool {
	if outputMode == "" {
		outputMode = "raw"
	}
	return &TellStoryTool{
		store:       store,
		api:         api,
		llmFallback: llmFallback,
		outputMode:  outputMode,
	}
}

// Name 返回工具名称
func (t *TellStoryTool) Name() string {
	return "tell_story"
}

// Description 返回工具描述
func (t *TellStoryTool) Description() string {
	return `讲一个故事。可以指定故事类型或关键词。
示例：
- "讲个故事" - 随机讲一个故事
- "讲个童话故事" - 讲童话故事
- "讲个小马过河的故事" - 讲指定标题的故事
- "讲个关于勇气的故事" - 根据主题找故事`
}

// Parameters 返回参数定义
func (t *TellStoryTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"keyword": {
				"type": "string",
				"description": "故事关键词（标题、类型或主题）"
			}
		}
	}`)
}

// Execute 执行讲故事
func (t *TellStoryTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params TellStoryParams
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	keyword := params.Keyword
	logger.Debugf("[story] 查找故事: %s", keyword)

	// 1. 本地库查找
	if t.store != nil {
		story, err := t.store.FindStory(keyword)
		if err == nil && story != nil {
			logger.Debugf("[story] 从本地库找到: %s (来源: %s, 播放次数: %d)", 
				story.Title, story.Source, story.PlayCount)
			return t.formatStoryResult(story), nil
		}
	}

	// 2. API 查找
	if t.api != nil {
		story, err := t.api.FindStory(ctx, keyword)
		if err == nil && story != nil {
			logger.Debugf("[story] 从 API 找到: %s (来源: %s)", story.Title, story.Source)
			// 保存到本地库
			if t.store != nil {
				t.store.SaveStory(story)
			}
			return t.formatStoryResult(story), nil
		}
		logger.Debugf("[story] API 查找失败: %v", err)
	}

	// 3. LLM 兜底提示
	if t.llmFallback {
		// 返回提示，让 LLM 自己生成故事（需要 LLM 处理，SkipLLM=false）
		result := StoryResult{
			Success: true,
			Content: fmt.Sprintf("本地故事库和外部API都没有找到相关故事。请你自己创作一个故事。关键词：%s。要求：故事要生动有趣，适合儿童听，长度适中（300-500字）。", keyword),
			SkipLLM: false,
		}
		data, _ := json.Marshal(result)
		return string(data), nil
	}

	// 未找到故事
	result := StoryResult{
		Success: false,
		Error:   "抱歉，我没有找到相关的故事。",
		SkipLLM: true, // 直接输出错误信息
	}
	data, _ := json.Marshal(result)
	return string(data), nil
}

// formatStoryResult 根据输出模式格式化故事结果
func (t *TellStoryTool) formatStoryResult(story *Story) string {
	content := fmt.Sprintf("《%s》\n\n%s", story.Title, story.Content)
	
	result := StoryResult{
		Success: true,
		Content: content,
		Title:   story.Title,
		SkipLLM: t.outputMode == "raw", // raw 模式跳过 LLM
	}
	
	data, _ := json.Marshal(result)
	return string(data)
}

// SaveStoryTool 保存故事工具（供用户主动保存 LLM 生成的故事）
type SaveStoryTool struct {
	store *StoryStore
}

// NewSaveStoryTool 创建保存故事工具
func NewSaveStoryTool(store *StoryStore) *SaveStoryTool {
	return &SaveStoryTool{store: store}
}

// Name 返回工具名称
func (t *SaveStoryTool) Name() string {
	return "save_story"
}

// Description 返回工具描述
func (t *SaveStoryTool) Description() string {
	return "保存一个故事到故事库。当用户说\"保存这个故事\"或\"记住这个故事\"时使用。"
}

// Parameters 返回参数定义
func (t *SaveStoryTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"title": {
				"type": "string",
				"description": "故事标题"
			},
			"category": {
				"type": "string",
				"description": "故事分类（如：童话故事、睡前故事、寓言故事）"
			},
			"content": {
				"type": "string",
				"description": "故事内容"
			},
			"tags": {
				"type": "array",
				"items": {"type": "string"},
				"description": "故事标签（如：勇气、友情、智慧）"
			}
		},
		"required": ["title", "content"]
	}`)
}

// SaveStoryParams 保存故事参数
type SaveStoryParams struct {
	Title    string   `json:"title"`
	Category string   `json:"category"`
	Content  string   `json:"content"`
	Tags     []string `json:"tags"`
}

// Execute 执行保存故事
func (t *SaveStoryTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params SaveStoryParams
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	if params.Title == "" || params.Content == "" {
		return "", fmt.Errorf("标题和内容不能为空")
	}

	if params.Category == "" {
		params.Category = "用户故事"
	}

	story := &Story{
		Title:    params.Title,
		Category: params.Category,
		Content:  params.Content,
		Tags:     params.Tags,
		Source:   "llm",
	}

	if err := t.store.SaveStory(story); err != nil {
		return "", fmt.Errorf("保存故事失败: %w", err)
	}

	return fmt.Sprintf("已保存故事《%s》到故事库", params.Title), nil
}

// ---- ListStoriesTool 列出故事 ----

// ListStoriesTool 列出故事工具
type ListStoriesTool struct {
	store *StoryStore
}

// NewListStoriesTool 创建列出故事工具
func NewListStoriesTool(store *StoryStore) *ListStoriesTool {
	return &ListStoriesTool{store: store}
}

// Name 返回工具名称
func (t *ListStoriesTool) Name() string {
	return "list_stories"
}

// Description 返回工具描述
func (t *ListStoriesTool) Description() string {
	return "列出可用的故事分类或某个分类下的故事列表"
}

// Parameters 返回参数定义
func (t *ListStoriesTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"category": {
				"type": "string",
				"description": "故事分类（可选，不填则列出所有分类）"
			},
			"limit": {
				"type": "integer",
				"description": "返回数量限制（默认10）"
			}
		}
	}`)
}

// ListStoriesParams 列出故事参数
type ListStoriesParams struct {
	Category string `json:"category"`
	Limit    int    `json:"limit"`
}

// Execute 执行列出故事
func (t *ListStoriesTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params ListStoriesParams
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	if params.Limit == 0 {
		params.Limit = 10
	}

	if params.Category == "" {
		// 列出所有分类
		categories := t.store.ListCategories()
		result := "可用的故事分类：\n"
		for _, cat := range categories {
			count := t.store.CountBySource(cat)
			result += fmt.Sprintf("- %s (%d 个故事)\n", cat, count)
		}
		llmCount := t.store.CountBySource("llm")
		if llmCount > 0 {
			result += fmt.Sprintf("\n用户保存的故事: %d 个\n", llmCount)
		}
		return result, nil
	}

	// 列出指定分类的故事
	stories := t.store.ListStories(params.Category, params.Limit)
	if len(stories) == 0 {
		return fmt.Sprintf("分类\"%s\" 下没有找到故事", params.Category), nil
	}

	result := fmt.Sprintf("分类\"%s\" 下的故事：\n", params.Category)
	for _, s := range stories {
		result += fmt.Sprintf("- 《%s》 (播放 %d 次)\n", s.Title, s.PlayCount)
	}
	return result, nil
}

// ---- DeleteStoryTool 删除故事 ----

// DeleteStoryTool 删除故事工具
type DeleteStoryTool struct {
	store *StoryStore
}

// NewDeleteStoryTool 创建删除故事工具
func NewDeleteStoryTool(store *StoryStore) *DeleteStoryTool {
	return &DeleteStoryTool{store: store}
}

// Name 返回工具名称
func (t *DeleteStoryTool) Name() string {
	return "delete_story"
}

// Description 返回工具描述
func (t *DeleteStoryTool) Description() string {
	return `删除故事库中的故事。可以删除用户保存的故事或API缓存的故事，但不能删除内置故事。
示例：
- "删除荆轲刺秦王这个故事" - 删除指定故事
- "删除所有用户保存的故事" - 删除所有 source=llm 的故事
- "清空故事缓存" - 删除所有 API 缓存的故事`
}

// Parameters 返回参数定义
func (t *DeleteStoryTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"keyword": {
				"type": "string",
				"description": "故事关键词（标题或模糊匹配）"
			},
			"delete_all_user": {
				"type": "boolean",
				"description": "删除所有用户保存的故事（source=llm）"
			},
			"clear_cache": {
				"type": "boolean",
				"description": "清空所有 API 缓存的故事（source=api）"
			}
		}
	}`)
}

// DeleteStoryParams 删除故事参数
type DeleteStoryParams struct {
	Keyword        string `json:"keyword"`
	DeleteAllUser  bool   `json:"delete_all_user"`
	ClearCache     bool   `json:"clear_cache"`
}

// Execute 执行删除故事
func (t *DeleteStoryTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params DeleteStoryParams
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	// 删除所有用户保存的故事
	if params.DeleteAllUser {
		count, err := t.store.DeleteAllUserStories()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("已删除 %d 个用户保存的故事", count), nil
	}

	// 清空 API 缓存
	if params.ClearCache {
		count, err := t.store.DeleteAllCachedStories()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("已清空 %d 个 API 缓存的故事", count), nil
	}

	// 根据关键词删除
	if params.Keyword != "" {
		count, err := t.store.DeleteByKeyword(params.Keyword)
		if err != nil {
			return "", err
		}
		if count == 0 {
			return "没有找到可以删除的故事（内置故事不能删除）", nil
		}
		return fmt.Sprintf("已删除 %d 个故事", count), nil
	}

	return "请指定要删除的故事关键词，或使用 delete_all_user/clear_cache 参数", nil
}

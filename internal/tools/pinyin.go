package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/mozillazg/go-pinyin"
)

// PinyinTool 汉字转拼音工具。
type PinyinTool struct {
	converter pinyin.Args
}

// NewPinyinTool 创建汉字转拼音工具。
func NewPinyinTool() *PinyinTool {
	// 默认使用带声调的拼音
	args := pinyin.NewArgs()
	args.Style = pinyin.Tone
	args.Heteronym = true // 支持多音字
	return &PinyinTool{converter: args}
}

// Name 返回工具名称。
func (t *PinyinTool) Name() string {
	return "pinyin_query"
}

// Description 返回工具描述。
func (t *PinyinTool) Description() string {
	return `查询汉字的拼音。返回汉字的读音，支持多音字和生僻字。
例如：
- "龘" -> dá
- "银行" -> yín háng
- "重庆" -> chóng qìng / zhòng qìng（多音字）`
}

// Parameters 返回工具参数定义。
func (t *PinyinTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"text": {
				"type": "string",
				"description": "要查询拼音的汉字文本"
			},
			"tone": {
				"type": "boolean",
				"description": "是否带声调，默认 true"
			}
		},
		"required": ["text"]
	}`)
}

// Execute 执行工具。
func (t *PinyinTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Text string `json:"text"`
		Tone *bool  `json:"tone"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	if params.Text == "" {
		return "", fmt.Errorf("请提供要查询拼音的汉字")
	}

	// 设置声调风格
	tone := true
	if params.Tone != nil {
		tone = *params.Tone
	}

	// 转换拼音
	result := t.convertToPinyin(params.Text, tone)
	return result, nil
}

// convertToPinyin 将文本转换为拼音。
func (t *PinyinTool) convertToPinyin(text string, withTone bool) string {
	// 设置声调风格
	style := pinyin.Tone
	if !withTone {
		style = pinyin.Normal
	}

	var result strings.Builder
	var lastWasHanzi bool

	for _, char := range text {
		if unicode.Is(unicode.Han, char) {
			// 是汉字
			args := pinyin.NewArgs()
			args.Style = style
			args.Heteronym = true
			pinyins := pinyin.Pinyin(string(char), args)

			if len(pinyins) > 0 && len(pinyins[0]) > 0 {
				if lastWasHanzi {
					result.WriteString(" ")
				}
				// 如果是多音字，用 / 分隔
				result.WriteString(strings.Join(pinyins[0], "/"))
			}
			lastWasHanzi = true
		} else if unicode.IsSpace(char) {
			// 空格保持
			result.WriteRune(char)
			lastWasHanzi = false
		} else {
			// 非汉字字符保持原样
			result.WriteRune(char)
			lastWasHanzi = false
		}
	}

	return result.String()
}

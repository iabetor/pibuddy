package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/iabetor/pibuddy/internal/logger"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tmt "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/tmt/v20180321"
)

// TranslateTool 腾讯云机器翻译工具。
type TranslateTool struct {
	client *tmt.Client
}

// NewTranslateTool 创建翻译工具。
func NewTranslateTool(secretID, secretKey, region string) (*TranslateTool, error) {
	credential := common.NewCredential(secretID, secretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "tmt.tencentcloudapi.com"

	client, err := tmt.NewClient(credential, region, cpf)
	if err != nil {
		return nil, fmt.Errorf("创建翻译客户端失败: %w", err)
	}

	logger.Info("[tools] 翻译工具已初始化")
	return &TranslateTool{client: client}, nil
}

func (t *TranslateTool) Name() string { return "translate" }
func (t *TranslateTool) Description() string {
	return "翻译文本。当用户要求翻译、说外语时使用。支持中、英、日、韩等多种语言。"
}
func (t *TranslateTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"text": {
				"type": "string",
				"description": "待翻译的文本"
			},
			"target_lang": {
				"type": "string",
				"description": "目标语言，如 zh(中文)、en(英语)、ja(日语)、ko(韩语)、fr(法语)、de(德语)、es(西班牙语)、ru(俄语)"
			},
			"source_lang": {
				"type": "string",
				"description": "源语言，可省略（自动检测）。常用值：zh、en、ja、ko"
			}
		},
		"required": ["text", "target_lang"]
	}`)
}

type translateArgs struct {
	Text       string `json:"text"`
	TargetLang string `json:"target_lang"`
	SourceLang string `json:"source_lang"`
}

// 语言代码映射（用户友好 -> 腾讯云代码）
var langCodeMap = map[string]string{
	"中文":     "zh",
	"汉语":     "zh",
	"英文":     "en",
	"英语":     "en",
	"日文":     "ja",
	"日语":     "ja",
	"韩文":     "ko",
	"韩语":     "ko",
	"法语":     "fr",
	"德语":     "de",
	"西班牙语": "es",
	"俄语":     "ru",
	"葡萄牙语": "pt",
	"意大利语": "it",
	"越南语":   "vi",
	"泰语":     "th",
	"阿拉伯语": "ar",
}

func (t *TranslateTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a translateArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	if a.Text == "" {
		return "", fmt.Errorf("翻译文本不能为空")
	}

	// 转换语言代码
	targetLang := a.TargetLang
	if code, ok := langCodeMap[targetLang]; ok {
		targetLang = code
	}

	// 源语言（默认自动检测）
	sourceLang := "auto"
	if a.SourceLang != "" {
		if code, ok := langCodeMap[a.SourceLang]; ok {
			sourceLang = code
		} else {
			sourceLang = a.SourceLang
		}
	}

	// 构建请求
	request := tmt.NewTextTranslateRequest()
	request.SourceText = common.StringPtr(a.Text)
	request.Source = common.StringPtr(sourceLang)
	request.Target = common.StringPtr(targetLang)
	request.ProjectId = common.Int64Ptr(0)

	// 调用 API
	response, err := t.client.TextTranslate(request)
	if err != nil {
		return "", fmt.Errorf("翻译请求失败: %w", err)
	}

	if response.Response == nil || response.Response.TargetText == nil {
		return "", fmt.Errorf("翻译响应为空")
	}

	result := *response.Response.TargetText
	detectedSource := ""
	if response.Response.Source != nil {
		detectedSource = *response.Response.Source
	}

	logger.Debugf("[tools] 翻译完成: %s -> %s, 结果: %s", detectedSource, targetLang, result)

	// 返回结果
	return fmt.Sprintf("%s", result), nil
}

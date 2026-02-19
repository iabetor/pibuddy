package tools

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestTranslateTool(t *testing.T) {
	// 从环境变量获取凭证
	secretID := os.Getenv("PIBUDDY_TENCENT_SECRET_ID")
	secretKey := os.Getenv("PIBUDDY_TENCENT_SECRET_KEY")

	if secretID == "" || secretKey == "" {
		t.Skip("跳过翻译测试: 未设置 PIBUDDY_TENCENT_SECRET_ID 或 PIBUDDY_TENCENT_SECRET_KEY")
	}

	tool, err := NewTranslateTool(secretID, secretKey, "ap-guangzhou")
	if err != nil {
		t.Fatalf("创建翻译工具失败: %v", err)
	}

	tests := []struct {
		name       string
		text       string
		targetLang string
		sourceLang string
		wantErr    bool
	}{
		{
			name:       "英译中",
			text:       "Hello, world!",
			targetLang: "zh",
			sourceLang: "en",
			wantErr:    false,
		},
		{
			name:       "中译英",
			text:       "你好，世界！",
			targetLang: "en",
			sourceLang: "zh",
			wantErr:    false,
		},
		{
			name:       "自动检测语言翻译",
			text:       "This is a test.",
			targetLang: "zh",
			sourceLang: "", // 自动检测
			wantErr:    false,
		},
		{
			name:       "中文语言名",
			text:       "你好",
			targetLang: "英语",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"text":        tt.text,
				"target_lang": tt.targetLang,
			}
			if tt.sourceLang != "" {
				args["source_lang"] = tt.sourceLang
			}

			argsJSON, _ := json.Marshal(args)
			result, err := tool.Execute(context.Background(), argsJSON)

			if (err != nil) != tt.wantErr {
				t.Errorf("TranslateTool.Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result == "" {
				t.Error("TranslateTool.Execute() 返回空结果")
			}

			t.Logf("翻译结果: %s -> %s", tt.text, result)
		})
	}
}

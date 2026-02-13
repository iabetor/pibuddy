package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// StockTool 查询 A 股实时行情。
type StockTool struct {
	client *http.Client
}

func NewStockTool() *StockTool {
	return &StockTool{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (t *StockTool) Name() string { return "get_stock" }

func (t *StockTool) Description() string {
	return "查询A股股票实时行情。当用户询问股票价格、股票涨跌等时使用。支持股票代码或名称。"
}

func (t *StockTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"code": {
				"type": "string",
				"description": "股票代码或名称，例如 000001 或 平安银行。上海股票前缀sh，深圳股票前缀sz。如果只传数字则自动判断。"
			}
		},
		"required": ["code"]
	}`)
}

type stockArgs struct {
	Code string `json:"code"`
}

func (t *StockTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a stockArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	code := strings.TrimSpace(a.Code)
	if code == "" {
		return "", fmt.Errorf("股票代码不能为空")
	}

	// 自动添加前缀，判断是否港股
	code, isHK := normalizeStockCode(code)

	return t.queryTencent(ctx, code, isHK)
}

// normalizeStockCode 规范化股票代码，返回 (code, isHK)。
// A股: sh/sz 前缀
// 港股: hk 前缀（需要特殊处理）
func normalizeStockCode(code string) (normalized string, isHK bool) {
	code = strings.ToLower(code)
	
	// 已经有前缀
	if strings.HasPrefix(code, "sh") || strings.HasPrefix(code, "sz") {
		return code, false
	}
	if strings.HasPrefix(code, "hk") {
		return code, true
	}
	
	// 港股：5位数字，通常以 0 开头
	if len(code) == 5 {
		return "hk" + code, true
	}
	
	// A股：6位数字
	if len(code) == 6 {
		switch {
		case strings.HasPrefix(code, "6"):
			return "sh" + code, false
		case strings.HasPrefix(code, "0") || strings.HasPrefix(code, "3"):
			return "sz" + code, false
		}
	}
	
	return code, false
}

func (t *StockTool) queryTencent(ctx context.Context, code string, isHK bool) (string, error) {
	var u string
	if isHK {
		// 港股：使用 web.sqt.gtimg.cn 和 r_ 前缀
		// hk00700 -> r_hk00700
		u = fmt.Sprintf("https://web.sqt.gtimg.cn/q=r_%s", code)
	} else {
		// A股
		u = fmt.Sprintf("https://qt.gtimg.cn/q=%s", code)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("查询股票失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	return parseTencentStock(string(body))
}

// parseTencentStock 解析腾讯股票数据。
// A股格式：v_sh600519="1~贵州茅台~600519~1483.93~..."
// 港股格式：v_r_hk00700="100~腾讯控股~00700~531.000~..."
// A股和港股字段位置相同：[31]=涨跌额, [32]=涨跌幅, [33]=最高, [34]=最低
func parseTencentStock(data string) (string, error) {
	// 找到引号中的内容
	start := strings.Index(data, "\"")
	end := strings.LastIndex(data, "\"")
	if start == -1 || end == -1 || start >= end {
		return "", fmt.Errorf("无法解析股票数据")
	}
	content := data[start+1 : end]
	if content == "" {
		return "", fmt.Errorf("未找到该股票，请检查股票代码")
	}

	parts := strings.Split(content, "~")
	if len(parts) < 35 {
		return "", fmt.Errorf("股票数据格式异常，字段数不足: %d", len(parts))
	}

	name := parts[1]      // 名称
	code := parts[2]      // 代码
	price := parts[3]     // 现价
	lastClose := parts[4] // 昨收
	open := parts[5]      // 今开
	change := parts[31]   // 涨跌额
	changePct := parts[32] // 涨跌幅
	high := parts[33]     // 最高
	low := parts[34]      // 最低

	return fmt.Sprintf("%s(%s): 现价 %s, 涨跌 %s (%s%%), 今开 %s, 最高 %s, 最低 %s, 昨收 %s",
		name, code, price, change, changePct, open, high, low, lastClose), nil
}

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
	"unicode/utf8"
)

func TestLunarDateTool_Metadata(t *testing.T) {
	tool := NewLunarDateTool()

	if tool.Name() != "get_lunar_date" {
		t.Errorf("Name() = %s, want get_lunar_date", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description() 不应为空")
	}

	params := tool.Parameters()
	if len(params) == 0 {
		t.Error("Parameters() 不应为空")
	}

	// 验证参数格式
	var paramSchema map[string]interface{}
	if err := json.Unmarshal(params, &paramSchema); err != nil {
		t.Errorf("Parameters() 不是有效的 JSON: %v", err)
	}
}

func TestLunarDateTool_Execute_Basic(t *testing.T) {
	tool := NewLunarDateTool()

	result, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	var lunarResult LunarResult
	if err := json.Unmarshal([]byte(result), &lunarResult); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	// 验证基本字段
	if lunarResult.SolarDate == "" {
		t.Error("SolarDate 不应为空")
	}

	if lunarResult.LunarDate == "" {
		t.Error("LunarDate 不应为空")
	}

	if lunarResult.YearGanzhi == "" {
		t.Error("YearGanzhi 不应为空")
	}

	if lunarResult.Zodiac == "" {
		t.Error("Zodiac 不应为空")
	}

	if lunarResult.MonthGanzhi == "" {
		t.Error("MonthGanzhi 不应为空")
	}

	if lunarResult.DayGanzhi == "" {
		t.Error("DayGanzhi 不应为空")
	}

	if lunarResult.Constellation == "" {
		t.Error("Constellation 不应为空")
	}

	// 默认不包含黄历信息
	if lunarResult.Huangli != nil {
		t.Error("默认不应包含黄历信息")
	}

	t.Logf("基本查询结果: %s", result)
}

func TestLunarDateTool_Execute_WithHuangli(t *testing.T) {
	tool := NewLunarDateTool()

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"include_huangli": true}`))
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	var lunarResult LunarResult
	if err := json.Unmarshal([]byte(result), &lunarResult); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	// 验证黄历信息
	if lunarResult.Huangli == nil {
		t.Fatal("应包含黄历信息")
	}

	if len(lunarResult.Huangli.Yi) == 0 {
		t.Log("宜事项可能为空（某些日子）")
	}

	if len(lunarResult.Huangli.Ji) == 0 {
		t.Log("忌事项可能为空（某些日子）")
	}

	if lunarResult.Huangli.Chong == "" {
		t.Error("冲信息不应为空")
	}

	if lunarResult.Huangli.Sha == "" {
		t.Error("煞信息不应为空")
	}

	t.Logf("黄历查询结果: %s", result)
}

func TestLunarDateTool_Execute_InvalidJSON(t *testing.T) {
	tool := NewLunarDateTool()

	// 无效的 JSON 参数应使用默认值
	result, err := tool.Execute(context.Background(), json.RawMessage(`invalid json`))
	if err != nil {
		t.Fatalf("Execute() 不应因无效 JSON 失败: %v", err)
	}

	var lunarResult LunarResult
	if err := json.Unmarshal([]byte(result), &lunarResult); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	// 应返回基本结果
	if lunarResult.LunarDate == "" {
		t.Error("应返回基本农历信息")
	}
}

func TestLunarDateTool_Execute_EmptyArgs(t *testing.T) {
	tool := NewLunarDateTool()

	// 空参数
	result, err := tool.Execute(context.Background(), json.RawMessage{})
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	var lunarResult LunarResult
	if err := json.Unmarshal([]byte(result), &lunarResult); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	// 应返回基本结果
	if lunarResult.LunarDate == "" {
		t.Error("应返回基本农历信息")
	}
}

func TestLunarDateTool_GanzhiAccuracy(t *testing.T) {
	tool := NewLunarDateTool()

	// 测试特定日期的干支准确性
	// 2026年2月15日应该是农历丙寅年腊月廿八
	// 注意：这个测试依赖于 lunar-go 库的准确性

	result, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	var lunarResult LunarResult
	if err := json.Unmarshal([]byte(result), &lunarResult); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	// 验证格式正确（不验证具体值，因为日期会变化）
	// 干支是 2 个汉字字符
	if utf8.RuneCountInString(lunarResult.YearGanzhi) != 2 {
		t.Errorf("YearGanzhi 应为 2 个字符, got %d", utf8.RuneCountInString(lunarResult.YearGanzhi))
	}

	if utf8.RuneCountInString(lunarResult.MonthGanzhi) != 2 {
		t.Errorf("MonthGanzhi 应为 2 个字符, got %d", utf8.RuneCountInString(lunarResult.MonthGanzhi))
	}

	if utf8.RuneCountInString(lunarResult.DayGanzhi) != 2 {
		t.Errorf("DayGanzhi 应为 2 个字符, got %d", utf8.RuneCountInString(lunarResult.DayGanzhi))
	}

	t.Logf("干支信息: 年=%s, 月=%s, 日=%s", lunarResult.YearGanzhi, lunarResult.MonthGanzhi, lunarResult.DayGanzhi)
}

func TestLunarDateTool_ZodiacAccuracy(t *testing.T) {
	tool := NewLunarDateTool()

	result, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	var lunarResult LunarResult
	if err := json.Unmarshal([]byte(result), &lunarResult); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	// 验证生肖是 12 生肖之一
	validZodiacs := []string{"鼠", "牛", "虎", "兔", "龙", "蛇", "马", "羊", "猴", "鸡", "狗", "猪"}
	valid := false
	for _, z := range validZodiacs {
		if lunarResult.Zodiac == z {
			valid = true
			break
		}
	}

	if !valid {
		t.Errorf("Zodiac = %s, 应该是 12 生肖之一", lunarResult.Zodiac)
	}

	t.Logf("生肖: %s", lunarResult.Zodiac)
}

func TestLunarDateTool_SolarTermInfo(t *testing.T) {
	tool := NewLunarDateTool()

	result, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	var lunarResult LunarResult
	if err := json.Unmarshal([]byte(result), &lunarResult); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	// 节气信息可能为空（如果当天不是节气日）
	t.Logf("节气信息: 当前=%s, 下一个=%s", lunarResult.SolarTerm, lunarResult.NextSolarTerm)
}

func TestLunarDateTool_FestivalsInfo(t *testing.T) {
	tool := NewLunarDateTool()

	result, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	var lunarResult LunarResult
	if err := json.Unmarshal([]byte(result), &lunarResult); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	// 节日信息可能为空（如果当天不是节日）
	if len(lunarResult.Festivals) > 0 {
		t.Logf("节日: %v", lunarResult.Festivals)
	} else {
		t.Log("今天不是传统节日或法定节假日")
	}
}

func TestLunarDateTool_Execute_WithDate(t *testing.T) {
	tool := NewLunarDateTool()

	// 查询 2025-01-29（农历除夕/春节前一天）
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"date": "2025-01-29", "include_huangli": true}`))
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	var lunarResult LunarResult
	if err := json.Unmarshal([]byte(result), &lunarResult); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	if lunarResult.SolarDate != "2025-01-29" {
		t.Errorf("SolarDate = %s, want 2025-01-29", lunarResult.SolarDate)
	}

	if lunarResult.Huangli == nil {
		t.Error("应包含黄历信息")
	}

	t.Logf("2025-01-29 农历: %s, 干支: %s年 %s月 %s日", lunarResult.LunarDate, lunarResult.YearGanzhi, lunarResult.MonthGanzhi, lunarResult.DayGanzhi)
}

func TestLunarDateTool_Execute_Tomorrow(t *testing.T) {
	tool := NewLunarDateTool()

	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	args := fmt.Sprintf(`{"date": "%s", "include_huangli": true}`, tomorrow)
	result, err := tool.Execute(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	var lunarResult LunarResult
	if err := json.Unmarshal([]byte(result), &lunarResult); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	if lunarResult.SolarDate != tomorrow {
		t.Errorf("SolarDate = %s, want %s", lunarResult.SolarDate, tomorrow)
	}

	if lunarResult.Huangli == nil {
		t.Error("应包含黄历信息")
	}

	t.Logf("明天(%s) 农历: %s", tomorrow, lunarResult.LunarDate)
}

func TestLunarDateTool_Execute_InvalidDate(t *testing.T) {
	tool := NewLunarDateTool()

	_, err := tool.Execute(context.Background(), json.RawMessage(`{"date": "not-a-date"}`))
	if err == nil {
		t.Error("应对无效日期返回错误")
	}
}

func TestLunarDateTool_SolarDateFormat(t *testing.T) {
	tool := NewLunarDateTool()

	result, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	var lunarResult LunarResult
	if err := json.Unmarshal([]byte(result), &lunarResult); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	// 验证公历日期格式
	expectedFormat := time.Now().Format("2006-01-02")
	if lunarResult.SolarDate != expectedFormat {
		t.Errorf("SolarDate = %s, want %s", lunarResult.SolarDate, expectedFormat)
	}
}

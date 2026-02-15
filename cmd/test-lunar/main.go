package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/iabetor/pibuddy/internal/tools"
)

func main() {
	// 创建农历工具
	tool := tools.NewLunarDateTool()

	fmt.Println("=== 农历查询工具测试 ===")
	fmt.Println()

	// 测试 1: 基本查询
	fmt.Println("1. 基本查询（不包含黄历）")
	result, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result)
	fmt.Println()

	// 测试 2: 包含黄历信息
	fmt.Println("2. 详细查询（包含黄历信息）")
	result, err = tool.Execute(context.Background(), json.RawMessage(`{"include_huangli": true}`))
	if err != nil {
		log.Fatal(err)
	}

	// 格式化输出
	var lunarResult map[string]interface{}
	json.Unmarshal([]byte(result), &lunarResult)

	fmt.Printf("公历日期: %s\n", lunarResult["solar_date"])
	fmt.Printf("农历日期: %s\n", lunarResult["lunar_date"])
	fmt.Printf("年干支: %s\n", lunarResult["year_ganzhi"])
	fmt.Printf("生肖: %s\n", lunarResult["zodiac"])
	fmt.Printf("月干支: %s\n", lunarResult["month_ganzhi"])
	fmt.Printf("日干支: %s\n", lunarResult["day_ganzhi"])
	fmt.Printf("星座: %s\n", lunarResult["constellation"])
	fmt.Printf("节气: %s\n", lunarResult["solar_term"])
	fmt.Printf("下一节气: %s\n", lunarResult["next_solar_term"])

	if huangli, ok := lunarResult["huangli"].(map[string]interface{}); ok {
		fmt.Println("\n黄历信息:")
		fmt.Printf("  宜: %v\n", huangli["yi"])
		fmt.Printf("  忌: %v\n", huangli["ji"])
		fmt.Printf("  冲: %v\n", huangli["chong"])
		fmt.Printf("  煞: %v\n", huangli["sha"])
	}

	fmt.Println()
	fmt.Println("=== 测试完成 ===")
}

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/6tail/lunar-go/calendar"
)

// LunarDateTool 农历查询工具。
type LunarDateTool struct{}

func NewLunarDateTool() *LunarDateTool {
	return &LunarDateTool{}
}

func (t *LunarDateTool) Name() string { return "get_lunar_date" }

func (t *LunarDateTool) Description() string {
	return "查询指定日期的农历日期和传统历法信息。当用户询问农历日期、干支纪年、生肖、节气、传统节日、黄历宜忌等问题时使用。支持查询今天、明天、任意日期。"
}

func (t *LunarDateTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"date": {
				"type": "string",
				"description": "要查询的公历日期，格式 YYYY-MM-DD，默认今天。例如查明天就传明天的日期。"
			},
			"include_huangli": {
				"type": "boolean",
				"description": "是否包含黄历信息（宜忌、冲煞等），默认false"
			}
		},
		"required": []
	}`)
}

// LunarResult 农历查询结果。
type LunarResult struct {
	SolarDate      string          `json:"solar_date"`
	LunarDate      string          `json:"lunar_date"`
	YearGanzhi     string          `json:"year_ganzhi"`
	Zodiac         string          `json:"zodiac"`
	MonthGanzhi    string          `json:"month_ganzhi"`
	DayGanzhi      string          `json:"day_ganzhi"`
	Constellation  string          `json:"constellation"`
	SolarTerm      string          `json:"solar_term"`
	NextSolarTerm  string          `json:"next_solar_term"`
	Festivals      []string        `json:"festivals,omitempty"`
	Huangli        *HuangliInfo    `json:"huangli,omitempty"`
}

// HuangliInfo 黄历信息。
type HuangliInfo struct {
	Yi   []string `json:"yi"`   // 宜
	Ji   []string `json:"ji"`   // 忌
	Chong string   `json:"chong"` // 冲
	Sha   string   `json:"sha"`   // 煞
}

func (t *LunarDateTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	// 解析参数
	var params struct {
		Date           string `json:"date"`
		IncludeHuangli bool   `json:"include_huangli"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			params.IncludeHuangli = false
		}
	}

	// 确定查询日期
	var targetDate time.Time
	if params.Date != "" {
		parsed, err := time.Parse("2006-01-02", params.Date)
		if err != nil {
			return "", fmt.Errorf("日期格式错误，请使用 YYYY-MM-DD 格式: %w", err)
		}
		targetDate = parsed
	} else {
		targetDate = time.Now()
	}

	solar := calendar.NewSolarFromDate(targetDate)
	lunar := solar.GetLunar()

	// 构建结果
	result := LunarResult{
		SolarDate:     targetDate.Format("2006-01-02"),
		LunarDate:     lunar.String(),
		YearGanzhi:    lunar.GetYearInGanZhi(),
		Zodiac:        lunar.GetYearShengXiao(),
		MonthGanzhi:   lunar.GetMonthInGanZhi(),
		DayGanzhi:     lunar.GetDayInGanZhi(),
		Constellation: solar.GetXingZuo(),
	}

	// 获取节气信息
	jieQi := lunar.GetJieQi()
	if jieQi != "" {
		result.SolarTerm = fmt.Sprintf("当前节气: %s", jieQi)
	} else {
		// 获取最近的节气
		prevJieQi := lunar.GetPrevJieQi()
		nextJieQi := lunar.GetNextJieQi()
		if prevJieQi != nil && nextJieQi != nil {
			result.SolarTerm = fmt.Sprintf("%s后", prevJieQi.GetName())
			result.NextSolarTerm = fmt.Sprintf("%s (%s)", nextJieQi.GetName(), nextJieQi.GetSolar().String())
		}
	}

	// 获取节日信息
	festivals := lunar.GetFestivals()
	if festivals != nil && festivals.Len() > 0 {
		result.Festivals = []string{}
		for e := festivals.Front(); e != nil; e = e.Next() {
			if str, ok := e.Value.(string); ok {
				result.Festivals = append(result.Festivals, str)
			}
		}
	}

	// 获取公历节日
	solarFestivals := solar.GetFestivals()
	if solarFestivals != nil && solarFestivals.Len() > 0 {
		if result.Festivals == nil {
			result.Festivals = []string{}
		}
		for e := solarFestivals.Front(); e != nil; e = e.Next() {
			if str, ok := e.Value.(string); ok {
				result.Festivals = append(result.Festivals, str)
			}
		}
	}

	// 可选：获取黄历信息
	if params.IncludeHuangli {
		// 获取宜忌
		yiList := lunar.GetDayYi()
		var yi []string
		if yiList != nil {
			for e := yiList.Front(); e != nil; e = e.Next() {
				if str, ok := e.Value.(string); ok {
					yi = append(yi, str)
				}
			}
		}

		jiList := lunar.GetDayJi()
		var ji []string
		if jiList != nil {
			for e := jiList.Front(); e != nil; e = e.Next() {
				if str, ok := e.Value.(string); ok {
					ji = append(ji, str)
				}
			}
		}

		huangli := &HuangliInfo{
			Yi:    yi,
			Ji:    ji,
			Chong: lunar.GetDayChongDesc(),
			Sha:   lunar.GetDaySha(),
		}
		result.Huangli = huangli
	}

	// 序列化结果
	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("序列化结果失败: %w", err)
	}

	return string(data), nil
}

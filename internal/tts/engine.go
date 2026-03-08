package tts

import (
	"context"
	"strings"
)

// Engine 定义语音合成后端接口。
type Engine interface {
	// Synthesize 将文本转换为音频。
	// 返回 float32 音频样本、采样率（Hz）和错误。
	Synthesize(ctx context.Context, text string) ([]float32, int, error)
}

// PreprocessText 预处理文本，删除不适合朗读的字符。
// 所有 TTS 引擎调用前应先使用此函数处理文本。
func PreprocessText(text string) string {
	// 删除 Markdown 格式符号
	text = strings.ReplaceAll(text, "**", "")  // 粗体
	text = strings.ReplaceAll(text, "__", "")  // 粗体
	text = strings.ReplaceAll(text, "*", "")   // 斜体
	text = strings.ReplaceAll(text, "_", "")   // 斜体下划线
	text = strings.ReplaceAll(text, "`", "")   // 代码
	text = strings.ReplaceAll(text, "~~", "")  // 删除线
	text = strings.ReplaceAll(text, "#", "")   // 标题

	// 智能处理 Markdown 表格：转换为口语化句子
	text = convertTableToSpeech(text)

	// 将温度符号转换为口语化文本（℃/°C → 摄氏度）
	text = strings.ReplaceAll(text, "℃", "摄氏度")
	text = strings.ReplaceAll(text, "°C", "摄氏度")
	text = strings.ReplaceAll(text, "°c", "摄氏度")

	// 删除省略号（中文和英文）
	text = strings.ReplaceAll(text, "……", "")
	text = strings.ReplaceAll(text, "...", "")
	text = strings.ReplaceAll(text, "…", "")

	// 清理多余的空格和换行
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(text)
}

// isTableSeparator 判断是否是表格分隔行（如 |---|---|）
func isTableSeparator(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.Contains(trimmed, "-") || !strings.Contains(trimmed, "|") {
		return false
	}
	// 移除竖线和空格后，只剩下 - 和 : 就是分隔行
	cleaned := strings.ReplaceAll(trimmed, "|", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "")
	cleaned = strings.ReplaceAll(cleaned, ":", "")
	return len(cleaned) == 0
}

// extractTablePart 从一行文本中提取表格部分和前后的非表格文本。
// LLM 经常把表格行和普通文本粘在一起，如 "天气如下：| 日期 | 星期 |"
// 或 "| 东北风 | 1-3级 |请注意以上信息仅供参考"
func extractTablePart(line string) (before, tablePart, after string) {
	// 找第一个 | 和最后一个 |
	firstPipe := strings.Index(line, "|")
	lastPipe := strings.LastIndex(line, "|")

	if firstPipe == -1 {
		return line, "", ""
	}

	before = strings.TrimSpace(line[:firstPipe])
	if firstPipe == lastPipe {
		// 只有一个 |，不是表格
		return line, "", ""
	}

	tablePart = strings.TrimSpace(line[firstPipe : lastPipe+1])
	if lastPipe+1 < len(line) {
		after = strings.TrimSpace(line[lastPipe+1:])
	}
	return
}

// parseTableRow 解析表格行中的单元格
func parseTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	// 去掉首尾的 |
	trimmed = strings.Trim(trimmed, "|")
	parts := strings.Split(trimmed, "|")
	var cells []string
	for _, p := range parts {
		cell := strings.TrimSpace(p)
		if cell != "" {
			cells = append(cells, cell)
		}
	}
	return cells
}

// convertTableToSpeech 将 Markdown 表格转换为口语化文本。
// 处理 LLM 输出中表格与普通文字粘在一起的情况。
func convertTableToSpeech(text string) string {
	lines := strings.Split(text, "\n")

	// 第一遍：分离表格行和普通文本，提取纯表格部分
	type parsedLine struct {
		before    string // 表格前的文字
		tablePart string // 表格部分（含竖线）
		after     string // 表格后的文字
		isTable   bool
		isSep     bool // 是否是分隔行
	}

	var parsed []parsedLine
	for _, line := range lines {
		if isTableSeparator(line) {
			parsed = append(parsed, parsedLine{isSep: true})
			continue
		}
		if !strings.Contains(line, "|") {
			parsed = append(parsed, parsedLine{before: line})
			continue
		}
		before, tp, after := extractTablePart(line)
		if tp == "" {
			parsed = append(parsed, parsedLine{before: line})
		} else {
			parsed = append(parsed, parsedLine{before: before, tablePart: tp, after: after, isTable: true})
		}
	}

	// 第二遍：收集连续的表格行，识别表头 + 数据行
	var result []string
	i := 0
	for i < len(parsed) {
		p := parsed[i]

		if !p.isTable && !p.isSep {
			if p.before != "" {
				result = append(result, p.before)
			}
			i++
			continue
		}

		// 收集连续的表格区域
		var tableRows []parsedLine
		var beforeText, afterText string

		for i < len(parsed) && (parsed[i].isTable || parsed[i].isSep) {
			if parsed[i].isSep {
				i++
				continue
			}
			if parsed[i].before != "" && len(tableRows) == 0 {
				beforeText = parsed[i].before
			}
			tableRows = append(tableRows, parsed[i])
			// 记住最后一行的 after
			if parsed[i].after != "" {
				afterText = parsed[i].after
			}
			i++
		}

		// 输出表格前的文字
		if beforeText != "" {
			result = append(result, beforeText)
		}

		if len(tableRows) >= 2 {
			// 第一行是表头，后面是数据行
			headers := parseTableRow(tableRows[0].tablePart)
			for _, row := range tableRows[1:] {
				cells := parseTableRow(row.tablePart)
				spoken := tableRowToSpeech(headers, cells)
				if spoken != "" {
					result = append(result, spoken)
				}
			}
		} else if len(tableRows) == 1 {
			cells := parseTableRow(tableRows[0].tablePart)
			result = append(result, strings.Join(cells, "，"))
		}

		// 输出表格后的文字
		if afterText != "" {
			result = append(result, afterText)
		}
	}

	return strings.Join(result, "\n")
}

// tableRowToSpeech 将表头和数据行组合为口语化句子。
// 例如：表头 [日期, 星期, 白天, 晚上, 最高温度, 最低温度, 风向, 风力]
// 数据 [3月9日, 星期一, 多云, 晴, 17℃, 3℃, 东北风, 1-3级]
// 输出："3月9日星期一，白天多云，晚上晴，最高温度17摄氏度，最低温度3摄氏度，东北风1-3级"
func tableRowToSpeech(headers, cells []string) string {
	if len(cells) == 0 {
		return ""
	}

	// 定义可以跳过表头直接输出值的字段（值本身就能说明意思的）
	skipHeader := map[string]bool{
		"日期": true, "星期": true,
		"风向": true, // "东北风"本身就包含"风"字
		"风力": true, // "1-3级"跟在风向后面就懂了
	}

	var parts []string
	for j, cell := range cells {
		if j >= len(headers) {
			parts = append(parts, cell)
			continue
		}
		header := headers[j]

		if skipHeader[header] {
			parts = append(parts, cell)
		} else {
			// "白天多云"、"最高温度17℃"、"风向东北风"
			parts = append(parts, header+cell)
		}
	}

	return strings.Join(parts, "，")
}

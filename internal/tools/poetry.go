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

// PoetryClient 诗词 API 客户端。
type PoetryClient struct {
	client  *http.Client
	apiKey  string
	baseURL string
}

// NewPoetryClient 创建诗词 API 客户端。
func NewPoetryClient(apiKey string) *PoetryClient {
	return &PoetryClient{
		client:  &http.Client{Timeout: 10 * time.Second},
		apiKey:  apiKey,
		baseURL: "https://api.66mz8.com",
	}
}

// PoetryDailyTool 每日一诗工具。
type PoetryDailyTool struct {
	client *PoetryClient
}

// NewPoetryDailyTool 创建每日一诗工具。
func NewPoetryDailyTool(apiKey string) *PoetryDailyTool {
	return &PoetryDailyTool{
		client: NewPoetryClient(apiKey),
	}
}

// Name 返回工具名称。
func (t *PoetryDailyTool) Name() string {
	return "poetry_daily"
}

// Description 返回工具描述。
func (t *PoetryDailyTool) Description() string {
	return `获取每日推荐古诗词。返回一首经典古诗词，包含标题、作者、正文和译文。`
}

// Parameters 返回工具参数定义。
func (t *PoetryDailyTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
}

// Execute 执行工具。
func (t *PoetryDailyTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	return t.getDailyPoetry()
}

// getDailyPoetry 获取每日一诗。
func (t *PoetryDailyTool) getDailyPoetry() (string, error) {
	// 诗词六六六 API - 每日推荐
	apiURL := fmt.Sprintf("%s/api/poetry/daily", t.client.baseURL)
	if t.client.apiKey != "" {
		apiURL += "?key=" + t.client.apiKey
	}

	resp, err := t.client.client.Get(apiURL)
	if err != nil {
		// API 失败时返回内置诗词
		return t.getFallbackPoetry(), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return t.getFallbackPoetry(), nil
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Title   string `json:"title"`
			Author  string `json:"author"`
			Content string `json:"content"`
			Explain string `json:"explain"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil || result.Code != 200 {
		return t.getFallbackPoetry(), nil
	}

	return fmt.Sprintf("《%s》%s\n%s\n\n译文：%s",
		result.Data.Title, result.Data.Author, result.Data.Content, result.Data.Explain), nil
}

// getFallbackPoetry 获取备用诗词（API 失败时）。
func (t *PoetryDailyTool) getFallbackPoetry() string {
	poems := []struct {
		title, author, content, explain string
	}{
		{"静夜思", "李白", "床前明月光，疑是地上霜。\n举头望明月，低头思故乡。", "明亮的月光洒在床前，好像地上泛起了一层白霜。我抬起头来，望着天上的明月，不由得低下头来，思念起故乡。"},
		{"春晓", "孟浩然", "春眠不觉晓，处处闻啼鸟。\n夜来风雨声，花落知多少。", "春天睡觉不知天亮，到处都能听到鸟儿的叫声。想起昨夜的风雨声，不知道花儿被打落了多少。"},
		{"登鹳雀楼", "王之涣", "白日依山尽，黄河入海流。\n欲穷千里目，更上一层楼。", "太阳靠着山头渐渐落下，黄河向着大海滚滚奔流。要想看到更远的风景，就要再登上更高的一层楼。"},
	}

	// 按日期选择
	day := time.Now().Day() % len(poems)
	p := poems[day]
	return fmt.Sprintf("《%s》%s\n%s\n\n译文：%s", p.title, p.author, p.content, p.explain)
}

// PoetrySearchTool 诗词搜索工具。
type PoetrySearchTool struct {
	client *PoetryClient
}

// NewPoetrySearchTool 创建诗词搜索工具。
func NewPoetrySearchTool(apiKey string) *PoetrySearchTool {
	return &PoetrySearchTool{
		client: NewPoetryClient(apiKey),
	}
}

// Name 返回工具名称。
func (t *PoetrySearchTool) Name() string {
	return "poetry_search"
}

// Description 返回工具描述。
func (t *PoetrySearchTool) Description() string {
	return `搜索古诗词。支持按关键词、作者或诗句搜索。
搜索类型：
- keyword: 按关键词搜索（如 "月亮"）
- author: 按作者搜索（如 "李白"）
- sentence: 按诗句搜索下一句（如 "春眠不觉晓"）`
}

// Parameters 返回工具参数定义。
func (t *PoetrySearchTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "搜索内容"
			},
			"type": {
				"type": "string",
				"enum": ["keyword", "author", "sentence"],
				"description": "搜索类型，默认 keyword"
			}
		},
		"required": ["query"]
	}`)
}

// Execute 执行工具。
func (t *PoetrySearchTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Query string `json:"query"`
		Type  string `json:"type"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	if params.Query == "" {
		return "", fmt.Errorf("请提供搜索内容")
	}

	if params.Type == "" {
		params.Type = "keyword"
	}

	return t.searchPoetry(params.Query, params.Type)
}

// searchPoetry 搜索诗词。
func (t *PoetrySearchTool) searchPoetry(query, searchType string) (string, error) {
	switch searchType {
	case "sentence":
		return t.searchNextLine(query)
	case "author":
		return t.searchByAuthor(query)
	default:
		return t.searchByKeyword(query)
	}
}

// searchByKeyword 按关键词搜索。
func (t *PoetrySearchTool) searchByKeyword(keyword string) (string, error) {
	// 内置诗词库搜索
	poems := t.searchLocalPoems(keyword)
	if len(poems) > 0 {
		var results []string
		for i, p := range poems {
			if i >= 3 {
				break // 最多返回 3 首
			}
			// 取第一句
			firstLine := strings.Split(p.Content, "\n")[0]
			results = append(results, fmt.Sprintf("《%s》%s - %s", p.Title, p.Author, firstLine))
		}
		return fmt.Sprintf("找到 %d 首包含 %q 的诗词：\n%s", len(poems), keyword, strings.Join(results, "\n")), nil
	}

	return fmt.Sprintf("未找到包含 %q 的诗词", keyword), nil
}

// searchByAuthor 按作者搜索。
func (t *PoetrySearchTool) searchByAuthor(author string) (string, error) {
	poems := t.getLocalPoemsByAuthor(author)
	if len(poems) > 0 {
		p := poems[0]
		return fmt.Sprintf("《%s》%s\n%s", p.Title, p.Author, p.Content), nil
	}

	return fmt.Sprintf("未找到作者 %q 的诗词", author), nil
}

// searchNextLine 查找下一句。
func (t *PoetrySearchTool) searchNextLine(sentence string) (string, error) {
	// 内置常见诗句映射
	nextLines := map[string]string{
		"春眠不觉晓":   "处处闻啼鸟",
		"床前明月光":   "疑是地上霜",
		"举头望明月":   "低头思故乡",
		"白日依山尽":   "黄河入海流",
		"欲穷千里目":   "更上一层楼",
		"鹅鹅鹅":     "曲项向天歌",
		"白毛浮绿水":   "红掌拨清波",
		"锄禾日当午":   "汗滴禾下土",
		"谁知盘中餐":   "粒粒皆辛苦",
		"离离原上草":   "一岁一枯荣",
		"野火烧不尽":   "春风吹又生",
		"日照香炉生紫烟": "遥看瀑布挂前川",
		"飞流直下三千尺": "疑是银河落九天",
		"两个黄鹂鸣翠柳": "一行白鹭上青天",
		"窗含西岭千秋雪": "门泊东吴万里船",
	}

	if next, ok := nextLines[sentence]; ok {
		// 查找完整诗信息
		for _, p := range t.getAllPoems() {
			if strings.Contains(p.Content, sentence) {
				return fmt.Sprintf("「%s」的下一句是「%s」，出自%s的《%s》", sentence, next, p.Author, p.Title), nil
			}
		}
		return fmt.Sprintf("「%s」的下一句是「%s」", sentence, next), nil
	}

	return fmt.Sprintf("未找到「%s」的下一句", sentence), nil
}

// PoetryGameTool 诗词游戏工具（飞花令/接龙）。
type PoetryGameTool struct {
	client  *PoetryClient
	session *GameSession
}

// GameSession 游戏会话。
type GameSession struct {
	GameType   string   // feihualing 或 jielong
	Keyword    string   // 飞花令关键字
	LastChar   string   // 接龙最后一个字
	UsedLines  []string // 已使用的诗句
	Score      int      // 得分
}

// NewPoetryGameTool 创建诗词游戏工具。
func NewPoetryGameTool(apiKey string) *PoetryGameTool {
	return &PoetryGameTool{
		client: NewPoetryClient(apiKey),
	}
}

// Name 返回工具名称。
func (t *PoetryGameTool) Name() string {
	return "poetry_game"
}

// Description 返回工具描述。
func (t *PoetryGameTool) Description() string {
	return `诗词游戏：飞花令或诗词接龙。
游戏类型：
- feihualing: 飞花令，轮流背含关键字的诗句
- jielong: 诗词接龙，上句尾字作下句首字

操作：
- start: 开始游戏，需指定 game 和 keyword（飞花令）
- respond: 回应诗句
- stop: 结束游戏`
}

// Parameters 返回工具参数定义。
func (t *PoetryGameTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["start", "respond", "stop"],
				"description": "操作类型"
			},
			"game": {
				"type": "string",
				"enum": ["feihualing", "jielong"],
				"description": "游戏类型（start 时必需）"
			},
			"keyword": {
				"type": "string",
				"description": "关键字（飞花令时必需）"
			},
			"line": {
				"type": "string",
				"description": "诗句（respond 时必需）"
			}
		},
		"required": ["action"]
	}`)
}

// Execute 执行工具。
func (t *PoetryGameTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Action  string `json:"action"`
		Game    string `json:"game"`
		Keyword string `json:"keyword"`
		Line    string `json:"line"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	switch params.Action {
	case "start":
		return t.startGame(params.Game, params.Keyword)
	case "respond":
		return t.respond(params.Line)
	case "stop":
		return t.stopGame()
	default:
		return "", fmt.Errorf("不支持的操作: %s", params.Action)
	}
}

// startGame 开始游戏。
func (t *PoetryGameTool) startGame(gameType, keyword string) (string, error) {
	if gameType == "" {
		return "", fmt.Errorf("请指定游戏类型：feihualing（飞花令）或 jielong（接龙）")
	}

	t.session = &GameSession{
		GameType:  gameType,
		Keyword:   keyword,
		UsedLines: []string{},
		Score:     0,
	}

	if gameType == "feihualing" {
		if keyword == "" {
			return "", fmt.Errorf("飞花令需要指定关键字")
		}
		// AI 先出一句
		line := t.findLineWithKeyword(keyword)
		t.session.UsedLines = append(t.session.UsedLines, line)
		return fmt.Sprintf("飞花令开始，关键字是「%s」！\n我先来：「%s」\n请接！", keyword, line), nil
	}

	// 接龙
	line := t.getRandomLine()
	t.session.LastChar = getLastChar(line)
	t.session.UsedLines = append(t.session.UsedLines, line)
	return fmt.Sprintf("诗词接龙开始！\n我先来：「%s」\n请接「%s」开头的诗句！", line, t.session.LastChar), nil
}

// respond 回应诗句。
func (t *PoetryGameTool) respond(line string) (string, error) {
	if t.session == nil {
		return "", fmt.Errorf("请先开始游戏")
	}

	if line == "" {
		return "", fmt.Errorf("请输入诗句")
	}

	// 检查是否已使用
	for _, used := range t.session.UsedLines {
		if used == line {
			return "这句已经用过了，请换一句！", nil
		}
	}

	// 验证诗句
	if t.session.GameType == "feihualing" {
		if !strings.Contains(line, t.session.Keyword) {
			return fmt.Sprintf("诗句中没有「%s」字，请重新接！", t.session.Keyword), nil
		}
	} else {
		// 接龙：检查首字
		firstChar := getFirstChar(line)
		if firstChar != t.session.LastChar {
			return fmt.Sprintf("首字应该是「%s」，你的是「%s」！", t.session.LastChar, firstChar), nil
		}
	}

	// 用户得分
	t.session.Score++
	t.session.UsedLines = append(t.session.UsedLines, line)

	// AI 回应
	var aiLine string
	if t.session.GameType == "feihualing" {
		aiLine = t.findLineWithKeyword(t.session.Keyword)
	} else {
		lastChar := getLastChar(line)
		aiLine = t.findLineStartingWith(lastChar)
		t.session.LastChar = getLastChar(aiLine)
	}

	if aiLine == "" {
		return fmt.Sprintf("厉害！我接不上了。游戏结束，你得了 %d 分！", t.session.Score), nil
	}

	t.session.UsedLines = append(t.session.UsedLines, aiLine)
	return fmt.Sprintf("好句！我接：「%s」\n该你了！", aiLine), nil
}

// stopGame 结束游戏。
func (t *PoetryGameTool) stopGame() (string, error) {
	if t.session == nil {
		return "当前没有进行中的游戏", nil
	}

	result := fmt.Sprintf("游戏结束！你得了 %d 分！", t.session.Score)
	t.session = nil
	return result, nil
}

// 辅助函数

func (t *PoetryGameTool) findLineWithKeyword(keyword string) string {
	for _, p := range t.getAllPoems() {
		if strings.Contains(p.Content, keyword) {
			// 检查是否已使用
			used := false
			for _, u := range t.session.UsedLines {
				if u == p.Content {
					used = true
					break
				}
			}
			if !used {
				return p.Content
			}
		}
	}
	return ""
}

func (t *PoetryGameTool) findLineStartingWith(char string) string {
	for _, p := range t.getAllPoems() {
		if getFirstChar(p.Content) == char {
			used := false
			for _, u := range t.session.UsedLines {
				if u == p.Content {
					used = true
					break
				}
			}
			if !used {
				return p.Content
			}
		}
	}
	return ""
}

func (t *PoetryGameTool) getRandomLine() string {
	poems := t.getAllPoems()
	if len(poems) > 0 {
		return poems[0].Content
	}
	return "床前明月光"
}

func (t *PoetryGameTool) getAllPoems() []PoemInfo {
	return []PoemInfo{
		{"静夜思", "李白", "床前明月光，疑是地上霜。"},
		{"静夜思", "李白", "举头望明月，低头思故乡。"},
		{"春晓", "孟浩然", "春眠不觉晓，处处闻啼鸟。"},
		{"春晓", "孟浩然", "夜来风雨声，花落知多少。"},
		{"登鹳雀楼", "王之涣", "白日依山尽，黄河入海流。"},
		{"登鹳雀楼", "王之涣", "欲穷千里目，更上一层楼。"},
		{"咏鹅", "骆宾王", "鹅鹅鹅，曲项向天歌。"},
		{"咏鹅", "骆宾王", "白毛浮绿水，红掌拨清波。"},
		{"悯农", "李绅", "锄禾日当午，汗滴禾下土。"},
		{"悯农", "李绅", "谁知盘中餐，粒粒皆辛苦。"},
		{"草", "白居易", "离离原上草，一岁一枯荣。"},
		{"草", "白居易", "野火烧不尽，春风吹又生。"},
		{"望庐山瀑布", "李白", "日照香炉生紫烟，遥看瀑布挂前川。"},
		{"望庐山瀑布", "李白", "飞流直下三千尺，疑是银河落九天。"},
		{"绝句", "杜甫", "两个黄鹂鸣翠柳，一行白鹭上青天。"},
		{"绝句", "杜甫", "窗含西岭千秋雪，门泊东吴万里船。"},
		{"相思", "王维", "红豆生南国，春来发几枝。"},
		{"相思", "王维", "愿君多采撷，此物最相思。"},
		{"九月九日忆山东兄弟", "王维", "独在异乡为异客，每逢佳节倍思亲。"},
		{"送元二使安西", "王维", "劝君更尽一杯酒，西出阳关无故人。"},
		{"水调歌头", "苏轼", "明月几时有，把酒问青天。"},
		{"水调歌头", "苏轼", "人有悲欢离合，月有阴晴圆缺。"},
		{"水调歌头", "苏轼", "但愿人长久，千里共婵娟。"},
	}
}

// PoemInfo 诗词信息。
type PoemInfo struct {
	Title   string
	Author  string
	Content string
}

func (t *PoetrySearchTool) getAllPoems() []PoemInfo {
	return []PoemInfo{
		{"静夜思", "李白", "床前明月光，疑是地上霜。\n举头望明月，低头思故乡。"},
		{"春晓", "孟浩然", "春眠不觉晓，处处闻啼鸟。\n夜来风雨声，花落知多少。"},
		{"登鹳雀楼", "王之涣", "白日依山尽，黄河入海流。\n欲穷千里目，更上一层楼。"},
		{"咏鹅", "骆宾王", "鹅鹅鹅，曲项向天歌。\n白毛浮绿水，红掌拨清波。"},
		{"悯农", "李绅", "锄禾日当午，汗滴禾下土。\n谁知盘中餐，粒粒皆辛苦。"},
		{"草", "白居易", "离离原上草，一岁一枯荣。\n野火烧不尽，春风吹又生。"},
		{"望庐山瀑布", "李白", "日照香炉生紫烟，遥看瀑布挂前川。\n飞流直下三千尺，疑是银河落九天。"},
		{"绝句", "杜甫", "两个黄鹂鸣翠柳，一行白鹭上青天。\n窗含西岭千秋雪，门泊东吴万里船。"},
		{"相思", "王维", "红豆生南国，春来发几枝。\n愿君多采撷，此物最相思。"},
		{"送元二使安西", "王维", "渭城朝雨浥轻尘，客舍青青柳色新。\n劝君更尽一杯酒，西出阳关无故人。"},
		{"水调歌头", "苏轼", "明月几时有，把酒问青天。\n不知天上宫阙，今夕是何年。"},
	}
}

func (t *PoetrySearchTool) searchLocalPoems(keyword string) []PoemInfo {
	var results []PoemInfo
	for _, p := range t.getAllPoems() {
		if strings.Contains(p.Content, keyword) || strings.Contains(p.Title, keyword) {
			results = append(results, p)
		}
	}
	return results
}

func (t *PoetrySearchTool) getLocalPoemsByAuthor(author string) []PoemInfo {
	var results []PoemInfo
	for _, p := range t.getAllPoems() {
		if strings.Contains(p.Author, author) {
			results = append(results, p)
		}
	}
	return results
}

func getFirstChar(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return ""
	}
	// 处理中文
	runes := []rune(s)
	return string(runes[0])
}

func getLastChar(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return ""
	}
	// 处理中文
	runes := []rune(s)
	return string(runes[len(runes)-1])
}

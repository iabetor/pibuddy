# Design: 音乐歌词显示

## 背景

用户希望在播放音乐时能看到歌词，后续可能通过树莓派外接屏幕实现独立显示。

## 目标 / 非目标

**目标：**
- 获取并解析 LRC 格式歌词
- 提供 HTTP API 供前端调用
- 支持后续扩展到树莓派屏幕显示

**非目标：**
- 歌词编辑功能
- 歌词上传/分享
- 实时翻译（后续扩展）

## 技术决策

### 1. 歌词数据模型

```go
// Lyric 表示一首歌曲的歌词
type Lyric struct {
    SongID   int64        // 歌曲 ID
    SongName string       // 歌曲名
    Artist   string       // 歌手
    Lines    []LyricLine  // 歌词行（按时间排序）
    Source   string       // 来源：qq/netease
}

// LyricLine 表示一行歌词
type LyricLine struct {
    Time    time.Duration // 时间戳
    Text    string        // 歌词文本
}
```

### 2. LRC 格式解析

标准 LRC 格式：
```
[ti:歌曲名]
[ar:歌手]
[al:专辑]
[00:15.50]故事的小黄花
[00:18.60]从出生那年就飘着
```

解析规则：
- 元数据标签：`[ti:]`、`[ar:]`、`[al:]`
- 时间标签：`[mm:ss.xx]` 或 `[mm:ss:xx]`
- 支持多时间标签：`[00:15.50][01:30.20]歌词`

### 3. Provider 接口扩展

```go
// LyricProvider 扩展接口，支持获取歌词
type LyricProvider interface {
    Provider
    GetLyric(ctx context.Context, songID int64, songMID string) (*Lyric, error)
}
```

### 4. 显示方案对比

| 方案 | 优点 | 缺点 | 适用场景 |
|------|------|------|---------|
| **Web UI** | 跨平台、开发简单 | 需要浏览器 | **推荐首选** |
| **Fyne GUI** | Go 原生、美观 | 需要桌面环境 | 树莓派 HDMI 显示屏 |
| **帧缓冲** | 资源占用低 | 开发复杂 | 小型 LCD/OLED |

### 5. 树莓派硬件选型

| 类型 | 尺寸 | 接口 | 价格 | 适合度 |
|------|------|------|------|--------|
| 官方触摸屏 | 7 英寸 | DSI | ¥300-400 | ⭐⭐⭐ 完整 UI |
| OLED 模块 | 0.96-1.5 英寸 | SPI/I2C | ¥20-50 | ⭐⭐ 仅显示当前行 |
| LCD1602 | 16x2 字符 | I2C | ¥15-30 | ⭐ 勉强可用 |
| 电子墨水屏 | 2-4 英寸 | SPI | ¥80-200 | ⭐⭐ 低功耗场景 |

**推荐路径**：先实现 Web UI，后续根据硬件选择集成方案。

### 6. API 设计

```
GET /api/lyric/current
Response: {
  "song_id": 123,
  "song_name": "晴天",
  "artist": "周杰伦",
  "current_line": "故事的小黄花",
  "next_line": "从出生那年就飘着",
  "lines": [
    {"time": 15.5, "text": "故事的小黄花"},
    {"time": 18.6, "text": "从出生那年就飘着"}
  ]
}
```

## 实现路径

### 阶段 1：歌词获取（后端）

1. 定义 `Lyric` 和 `LyricLine` 数据结构
2. 实现 LRC 格式解析器
3. 在 `Provider` 接口添加 `GetLyric` 方法
4. QQ 音乐客户端实现歌词接口
5. 网易云音乐客户端实现歌词接口

### 阶段 2：Web UI 显示

1. 添加 HTTP API 端点 `/api/lyric/current`
2. 前端页面实时轮询歌词
3. 歌词高亮当前行
4. 歌词滚动动画

### 阶段 3：树莓派集成

**方案 A：Chromium Kiosk**
```bash
chromium-browser --kiosk --app=http://localhost:8080/lyric
```

**方案 B：Fyne GUI**
```go
// 独立的歌词显示窗口
func showLyricWindow(lyric *Lyric) {
    a := app.New()
    w := a.NewWindow("Lyrics")
    w.SetContent(widget.NewLabel(lyric.CurrentLine()))
    w.ShowAndRun()
}
```

## 缓存策略

歌词获取后本地缓存，减少 API 调用：

```
{data_dir}/lyrics/
├── qq_0012345.lrc
├── netease_67890.lrc
└── ...
```

缓存规则：
- 缓存有效期：永久（歌词内容不变）
- 缓存键：`{provider}_{song_id}.lrc`

## 开放问题

1. 是否需要支持歌词翻译显示？
2. 树莓派屏幕选择哪种类型？
3. 是否需要离线歌词支持？

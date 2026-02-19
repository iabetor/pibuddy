# Project Context

## Purpose

PiBuddy 是一个运行在树莓派 4B 上的智能语音助手，通过唤醒词激活，流式语音识别用户输入，调用大模型生成回复，再通过 TTS 合成语音播放。目标是把树莓派变成一个由大模型驱动的智能音箱。

## Core Capabilities

### 语音交互
- **唤醒词检测**：基于 sherpa-onnx 的关键词检测，支持自定义唤醒词"你好小派"
- **语音识别 (ASR)**：流式中英双语识别 (Zipformer 模型)，实时输出识别结果
- **语音合成 (TTS)**：支持腾讯云 TTS（国内推荐）、Edge TTS（国际）、Piper TTS（离线）
- **打断机制**：在播放回复时可通过唤醒词打断，支持连续对话模式

### Function Calling & Tools
内置 20+ 工具，支持 LLM Function Calling：

| 工具类别 | 工具名称 | 功能描述 |
|----------|----------|----------|
| 日期时间 | `get_datetime` | 当前日期时间、星期 |
| 农历查询 | `get_lunar_date` | 农历日期、干支纪年、生肖、节气、黄历宜忌 |
| 天气查询 | `get_weather` | 实时天气、3/7/15 天预报 |
| 空气质量 | `get_air_quality` | 实时 AQI、污染物、健康建议 |
| 计算器 | `calculate` | 数学表达式求值 |
| 闹钟提醒 | `alarm` | 创建/查询/删除闹钟，到时语音播报 |
| 备忘录 | `memo` | 创建/查询/删除备忘录 |
| 新闻播报 | `get_news` | 获取热点新闻摘要 |
| 股票行情 | `get_stock` | A 股实时行情查询 |

### 音乐播放
- **多平台支持**：网易云音乐、QQ音乐
- **语音点歌**：说"播放XXX"自动搜索并播放
- **播放控制**：下一首、播放模式切换（顺序/循环/单曲）
- **本地缓存**：自动缓存已播放歌曲，支持离线播放
- **模糊搜索**：支持按歌名、歌手名搜索本地缓存

### RSS 订阅
- **订阅管理**：语音添加/删除/列出 RSS 订阅源
- **内容播报**：获取最新订阅内容，按来源或关键词过滤
- **缓存机制**：30 分钟缓存，减少重复请求

### 声纹识别
- **用户识别**：基于 sherpa-onnx Speaker Embedding，每次对话前自动识别说话人
- **个性化回复**：根据用户偏好调整回复风格，支持设置回复风格、兴趣爱好、昵称等
- **用户管理**：命令行工具注册/删除/列出用户，支持设置主人和用户偏好

#### 个性化回复设置

用户偏好字段：
- `style`: 回复风格（如"简洁直接"）
- `interests`: 兴趣爱好数组
- `nickname`: 昵称
- `extra`: 额外描述

设置方式：
```bash
# 命令行设置
./bin/pibuddy-user set-prefs 小明 '{"style":"简洁直接","interests":["编程"]}'

# 查看偏好
./bin/pibuddy-user get-prefs 小明
```

主人可通过语音对话设置偏好（需先 `./bin/pibuddy-user set-owner`）。

## Tech Stack

- **语言**: Go 1.21+, CGO enabled
- **音频 I/O**: miniaudio (通过 `gen2brain/malgo` Go 绑定), ALSA 后端
- **语音处理**: sherpa-onnx-go (唤醒词 KWS, VAD Silero, 流式 ASR Zipformer, 声纹提取)
- **LLM**: OpenAI 兼容协议 (SSE streaming), 支持任意兼容 API，Function Calling
- **TTS**: 腾讯云 TTS (在线, 国内推荐) / Edge TTS (在线) / Piper TTS (离线)
- **MP3 解码**: `hajimehoshi/go-mp3`
- **RSS 解析**: `github.com/mmcdole/gofeed`
- **农历计算**: `github.com/6tail/lunar-go`
- **数据库**: SQLite (modernc.org/sqlite，纯 Go 实现)
- **配置**: YAML (`gopkg.in/yaml.v3`), 支持 `${ENV_VAR}` 展开
- **构建**: Makefile, 支持本地编译和 ARM64 交叉编译
- **部署**: systemd service, SSH/SCP 一键部署

## Project Conventions

### Code Style

- 中文注释和日志
- 包级文档注释使用 `//` 格式
- 错误信息统一使用中文带组件前缀, 如 `[pipeline]`, `[llm]`, `[state]`
- 函数命名遵循 Go 标准: 导出函数 PascalCase, 私有函数 camelCase
- 配置结构体使用 `yaml` tag

### Architecture Patterns

- **Pipeline 模式**: `internal/pipeline/pipeline.go` 为主编排器, 串联所有组件
- **状态机**: 四状态 `Idle → Listening → Processing → Speaking → Idle`, 支持任意状态 → Idle (打断/错误恢复)
- **接口抽象**: LLM (`Provider` 接口) 和 TTS (`Engine` 接口) 通过接口解耦
- **流式处理**: LLM 响应通过 channel 逐块传递, 按句拆分后逐句 TTS + 播放
- **音频格式**: 内部统一使用 float32 [-1.0, 1.0], 通过 `internal/audio/convert.go` 在 int16/bytes/float32 间转换
- **工具注册表**: `internal/tools/registry.go` 管理所有工具，支持动态注册

### Testing Strategy

- 只使用标准库 `testing`, 不引入第三方测试框架
- 测试文件放在对应包目录下 (`_test.go`)
- 纯逻辑组件直接测试 (audio convert, context manager, state machine, sentence split, config)
- 网络依赖用 `net/http/httptest` mock (OpenAI SSE client, RSS fetcher, music provider)
- 所有测试必须在无硬件 (无麦克风/扬声器/模型文件) 的开发机上通过
- 运行命令: `make test` 或 `go test ./...`

### Git Workflow

- 主分支开发
- Makefile 驱动构建和部署

## Domain Context

- 目标硬件: 树莓派 4B (4GB+), ARM64 架构
- 音频采集: 16kHz 单声道 (麦克风通过 ALSA 驱动, 支持 USB 和 I2S)
- 音频播放: 24kHz 单声道 (支持 3.5mm/USB/蓝牙音箱, 蓝牙通过 PulseAudio A2DP)
- 蓝牙音箱可用于播放但其麦克风不可用 (HFP 8kHz 不适合 ASR)
- sherpa-onnx 模型需要单独下载, 不包含在代码仓库中
- LLM API 需要网络和有效 API Key

## Important Constraints

- CGO 必须启用 (malgo, sherpa-onnx 依赖 C 库)
- 交叉编译需要 `aarch64-linux-gnu-gcc`
- sherpa-onnx C 库需要在目标平台安装
- 树莓派板载蓝牙信号弱, 蓝牙音箱建议距离 < 3m
- 腾讯云 TTS / Edge TTS 需要互联网连接; Piper TTS 可离线但需要模型文件

## External Dependencies

| 依赖 | 用途 | 网络要求 |
|------|------|----------|
| OpenAI 兼容 API | 大模型对话 + Function Calling | 需要网络 |
| 和风天气 API | 天气查询、空气质量 | 需要网络 |
| 腾讯云 TTS | 在线语音合成（国内推荐） | 需要网络 |
| Edge TTS (Microsoft) | 在线语音合成（国际） | 需要网络 |
| Piper TTS | 离线语音合成 | 不需要 (本地模型) |
| sherpa-onnx models | 唤醒词/VAD/ASR/声纹推理 | 首次下载需要网络 |
| 网易云/QQ音乐 API | 音乐搜索和播放 | 需要网络 |

## Directory Structure

```
pibuddy/
├── cmd/                    # 命令行工具
│   ├── main.go            # 主程序入口
│   ├── music/main.go      # 音乐登录工具 (pibuddy-music)
│   └── user/main.go       # 用户管理工具 (pibuddy-user)
├── configs/                # 配置文件
│   └── pibuddy.yaml       # 主配置文件
├── internal/               # 内部包
│   ├── asr/               # 语音识别 (sherpa-onnx)
│   ├── audio/             # 音频 I/O、流播放、缓存
│   ├── config/            # 配置解析
│   ├── llm/               # LLM 客户端 (OpenAI 兼容)
│   ├── logger/            # 日志
│   ├── music/             # 音乐服务 (网易云/QQ音乐)
│   ├── pipeline/          # 主编排器
│   ├── rss/               # RSS 订阅管理
│   ├── tools/             # LLM 工具集
│   ├── tts/               # TTS 引擎
│   ├── vad/               # 语音活动检测
│   ├── voiceprint/        # 声纹识别
│   └── wake/              # 唤醒词检测
├── models/                 # 模型文件目录 (需下载)
├── scripts/                # 部署脚本
├── openspec/               # OpenSpec 规范文档
│   ├── project.md         # 本文件
│   ├── AGENTS.md          # AI 助手指引
│   ├── changes/           # 变更提案
│   │   ├── archive/       # 已归档的变更
│   │   └── */             # 进行中的变更
│   └── specs/             # 能力规格
└── Makefile               # 构建脚本
```

## Key Configuration

配置文件 `configs/pibuddy.yaml` 主要配置项：

```yaml
# 音频配置
audio:
  sample_rate: 16000        # 采样率
  frames_per_buffer: 512    # 每帧采样数

# 对话配置
dialog:
  wake_reply: "我在"        # 唤醒回复语
  interrupt_reply: "我在"   # 打断回复语
  listen_delay: 500         # 回复后延迟进入监听 (ms)
  continuous_timeout: 15    # 连续对话超时 (秒)

# LLM 配置
llm:
  api_base: "https://api.openai.com/v1"
  model: "gpt-4o-mini"

# TTS 配置
tts:
  engine: "tencent"         # tencent | edge | piper

# 音乐配置
music:
  enabled: true
  provider: "qqmusic"       # netease | qqmusic
  cache_max_size: 500       # 缓存上限 (MB)

# 声纹配置
voiceprint:
  enabled: true

# RSS 配置
rss:
  enabled: true
  cache_ttl: 30             # 缓存时间 (分钟)
```

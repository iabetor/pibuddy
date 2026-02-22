# PiBuddy — 树莓派智能语音助手

把树莓派 4B 变成一个比小爱同学更聪明的智能音箱，用大模型驱动对话。

## 核心功能

### 语音交互
- **语音唤醒**：说"你好小派"唤醒，支持自定义唤醒词
- **流式语音识别**：中英双语 ASR (sherpa-onnx Zipformer)，实时输出识别结果
- **多引擎 TTS**：腾讯云 TTS（国内推荐）、Edge TTS（国际）、Piper TTS（离线）
- **打断与连续对话**：播放时说唤醒词可打断，支持连续对话模式

### 智能工具 (25+)
通过 Function Calling 支持丰富的语音操控：

| 类别 | 功能示例 |
|------|----------|
| 📅 日期时间 | "今天星期几"、"现在几点了" |
| 🏮 农历查询 | "今天农历几号"、"今年是什么生肖年"、"今天宜做什么" |
| 🌤️ 天气查询 | "武汉天气怎么样"、"未来一周天气" |
| 🌬️ 空气质量 | "今天空气质量怎么样" |
| 🧮 计算器 | "23乘以45等于多少" |
| ⏰ 闹钟提醒 | "十分钟后提醒我关火"、"查看我的闹钟" |
| 📝 备忘录 | "记一下明天要带伞"、"查看备忘录" |
| 📰 新闻播报 | "有什么新闻" |
| 📈 股票行情 | "贵州茅台股价多少" |
| 📚 讲故事 | "讲个小马过河的故事"、"讲个睡前故事" |
| 🏠 智能家居 | "打开客厅灯"、"把空调调到26度" |
| 🌐 翻译 | "把你好翻译成英语" |
| 💊 健康提醒 | "提醒我每小时站起来活动" |
| 🎓 学习工具 | "每日一句英语"、"飞花令"、"诗词接龙" |

### 音乐播放
- **语音点歌**：说"播放小星星"、"我想听周杰伦的歌"
- **多平台支持**：网易云音乐、QQ音乐
- **播放控制**：下一首、播放模式切换（顺序/循环/单曲）
- **本地缓存**：自动缓存已播放歌曲，支持离线播放
- **智能搜索**：按歌手+歌名搜索，优先匹配指定歌手版本

### RSS 订阅
- **语音订阅**："订阅 XXX 网站的 RSS"
- **内容播报**："有什么新消息"、"看看科技资讯"
- **按来源/关键词过滤**："看看 RSS 里关于 AI 的内容"

### 讲故事
- **内置故事库**：58 个经典故事（童话、寓言、成语故事等）
- **外部 API 扩展**：mxnzp 故事大全 API
- **LLM 兜底**：定制化故事生成
- **零 Token 模式**：原文朗读，完全绕过 LLM

语音示例：
```
"讲个小马过河的故事"
"讲个睡前故事"
"讲一个关于勇气的故事"
"简单讲一下荆轲刺秦王的故事"  # LLM 总结版
```

### 声纹识别
- **自动识别说话人**：每次对话前自动识别用户身份
- **个性化回复**：根据用户身份提供个性化服务
- **多用户管理**：支持注册、删除用户

## 硬件要求

| 硬件 | 推荐 | 预算 | 备注 |
|------|------|------|------|
| 树莓派 | 4B (4GB+) | — | 需要 ARM64 架构 |
| 麦克风 | ReSpeaker 2-Mics HAT / USB 全向麦克风 | ¥30-80 | 必须支持 16kHz 单声道采集 |
| 扬声器 | 3.5mm 有源小音箱 / USB 音箱 / 蓝牙音箱 | ¥20-50 | 见下方蓝牙音箱说明 |
| 散热 | 散热片 + 小风扇（推荐） | ¥10 | — |

### 音频技术参数

代码对音频硬件的具体要求：

- **音频采集**: 16kHz, 单声道, 每帧 512 样本 (`internal/audio/capture.go`)
- **音频播放**: 24kHz, 单声道 (`internal/pipeline/pipeline.go`)
- **驱动层**: miniaudio (malgo), 在树莓派上通过 ALSA 后端工作

### 麦克风选型

麦克风需要被 ALSA 识别（`arecord -l` 可列出），有两种方案：

| 方案 | 优点 | 缺点 |
|------|------|------|
| **USB 麦克风** | 即插即用，无需额外配置 | 占用 USB 口 |
| **I2S 麦克风模块**（如 INMP441） | 不占 USB 口，延迟低 | 需要修改设备树配置 |

### 扬声器选型 / 蓝牙音箱

扬声器有三种接入方式：

| 方案 | 说明 |
|------|------|
| **3.5mm 有源音箱** | 接树莓派 3.5mm 口，最简单 |
| **USB 音箱** | 即插即用 |
| **蓝牙音箱** | 可行，但有注意事项（见下文） |

**蓝牙音箱注意事项**:

PiBuddy 通过 miniaudio → ALSA/PulseAudio 播放音频，蓝牙音箱只要在系统层面被识别为默认输出设备即可，代码无需修改。但需注意：

1. **延迟**: 蓝牙 A2DP 协议有 100-200ms 延迟，语音对话会有感知
2. **采样率重采样**: 代码输出 24kHz，蓝牙音箱通常接受 44.1kHz/48kHz，系统会自动重采样
3. **连接稳定性**: 树莓派板载蓝牙信号较弱，建议距离 < 3m
4. **麦克风仍需单独购买**: 蓝牙音箱的麦克风走 HFP 协议（8kHz），质量差且与 A2DP 不能同时使用，不适合语音识别

蓝牙音箱配对步骤：

```bash
sudo apt install bluez pulseaudio-module-bluetooth
bluetoothctl
  scan on
  pair <MAC地址>
  connect <MAC地址>
  trust <MAC地址>

# 确认音箱出现在音频输出
pactl list sinks short
```

## 快速开始（树莓派）

### 1. 在开发机上编译

```bash
# 本地编译
make build

# 交叉编译 ARM64（需要 aarch64-linux-gnu-gcc）
make build-arm64
```

### 2. 部署到树莓派

```bash
# 一键部署（编译 + 拷贝 + 安装服务）
make deploy PI=pi@raspberrypi.local
```

### 3. 在树莓派上初始化

```bash
# SSH 到树莓派
ssh pi@raspberrypi.local

# 运行初始化脚本（安装依赖、下载模型）
cd /home/pi/pibuddy
bash scripts/setup.sh
```

### 4. 配置 API Key

```bash
# 编辑环境变量文件
nano ~/.bashrc  # 或 ~/.zshrc

# LLM Provider Keys（多 Provider 支持，按顺序自动降级）
export PIBUDDY_QWEN_API_KEY='your-qwen-key'          # 通义千问 (推荐，免费400万token)
export PIBUDDY_HUNYUAN_API_KEY='your-hunyuan-key'    # 腾讯混元 (免费100万token)
export PIBUDDY_ARK_API_KEY='your-ark-key'            # 火山方舟 (免费50万token)
export PIBUDDY_ARK_ENDPOINT_ID='your-endpoint-id'    # 火山方舟接入点ID
export PIBUDDY_LLM_API_KEY='your-deepseek-key'       # DeepSeek (付费兜底)

# 腾讯云服务 (TTS, ASR, 翻译)
export PIBUDDY_TENCENT_SECRET_ID='your-secret-id'
export PIBUDDY_TENCENT_SECRET_KEY='your-secret-key'
export PIBUDDY_TENCENT_APP_ID='your-app-id'

# 其他服务
export PIBUDDY_QWEATHER_API_KEY='your-qweather-key'  # 和风天气
export PIBUDDY_HA_TOKEN='your-homeassistant-token'   # Home Assistant
export PIBUDDY_EZVIZ_AK='your-ezviz-ak'              # 萤石门锁
export PIBUDDY_EZVIZ_SK='your-ezviz-sk'

# 使环境变量生效
source ~/.bashrc
```

也可以直接编辑配置文件：
```bash
nano /home/pi/pibuddy/configs/pibuddy.yaml
```

### 5. 启动

```bash
# 直接运行
./pibuddy -config configs/pibuddy.yaml

# 或使用 systemd 服务（开机自启）
sudo systemctl enable --now pibuddy

# 查看日志
journalctl -u pibuddy -f
```

## 在 Mac 上测试

无需树莓派和外接硬件，可直接在 Mac 上运行完整语音交互来验证功能。Mac 内置的麦克风和扬声器通过 miniaudio 的 CoreAudio 后端工作，sherpa-onnx 的 Go 绑定已内置 macOS 预编译库（`sherpa-onnx-go-macos`），无需手动安装任何 C 依赖。

### 一键初始化

```bash
bash scripts/setup-mac.sh
```

脚本会自动完成：检查 Go 环境 → 下载全部模型 → 编译项目 → 检测音频设备。

### 配置 LLM

编辑 `configs/pibuddy.yaml`，设置你的 LLM 后端。支持多 Provider 自动降级：

```yaml
llm:
  # 多模型优先级列表，按顺序尝试，额度用完自动切换到下一个
  models:
    # 通义千问（免费额度最大，推荐）
    - name: "qwen-turbo"
      api_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
      api_key: "${PIBUDDY_QWEN_API_KEY}"
      model: "qwen-turbo"
    # 腾讯混元
    - name: "hunyuan-lite"
      api_url: "https://api.hunyuan.cloud.tencent.com/v1"
      api_key: "${PIBUDDY_HUNYUAN_API_KEY}"
      model: "hunyuan-lite"
    # 火山方舟（豆包）
    - name: "doubao"
      api_url: "https://ark.cn-beijing.volces.com/api/v3"
      api_key: "${PIBUDDY_ARK_API_KEY}"
      model: "${PIBUDDY_ARK_ENDPOINT_ID}"
    # DeepSeek（付费兜底）
    - name: "deepseek-chat"
      api_url: "https://api.deepseek.com/v1"
      api_key: "${PIBUDDY_LLM_API_KEY}"
      model: "deepseek-chat"
```

### 运行

```bash
export PIBUDDY_LLM_API_KEY='your-key'
./bin/pibuddy -config configs/pibuddy.yaml
```

说"你好小派"即可开始对话。

### Mac 测试注意事项

| 事项 | 说明 |
|------|------|
| 麦克风权限 | 首次运行时 macOS 会弹出麦克风授权弹窗，需要允许 |
| 音频设备 | 自动使用系统默认输入/输出设备，无需配置 |
| 外接蓝牙耳机 | 播放可以走蓝牙；但录音建议用 Mac 内置麦克风，蓝牙麦克风采样率低（8kHz HFP）不适合 ASR |
| 与树莓派的差异 | 代码完全一致，仅 miniaudio 后端不同（Mac: CoreAudio, 树莓派: ALSA） |

## 音乐服务配置

### QQ 音乐（推荐）

1. 部署 [QQAFLMusicApi](https://github.com/QiuChenlyOpenSource/QQAFLMusicApi) 服务
2. 配置 `pibuddy.yaml`：

```yaml
music:
  enabled: true
  provider: "qqmusic"
  qqmusic_api_url: "http://localhost:3300"
```

### 网易云音乐

1. 部署 [NeteaseCloudMusicApi](https://gitlab.com/Binaryify/NeteaseCloudMusicApi) 服务
2. 使用登录工具：

```bash
# 构建
make build-music

# 登录（会提示在浏览器扫码）
./bin/pibuddy-music login

# 查看登录状态
./bin/pibuddy-music status
```

3. 配置 `pibuddy.yaml`：

```yaml
music:
  enabled: true
  provider: "netease"
  netease_api_url: "http://localhost:3000"
```

## 声纹识别与个性化回复

### 注册用户声纹

```bash
# 构建用户管理工具
make build-user

# 注册用户（需要录制 3 个 3 秒语音样本）
./bin/pibuddy-user register 小明

# 查看已注册用户
./bin/pibuddy-user list

# 删除用户
./bin/pibuddy-user delete 小明
```

### 设置个性化偏好

每位用户可以设置偏好，系统会在对话时自动识别用户身份，并根据偏好调整回复风格。

**方式一：命令行工具**

```bash
# 设置用户偏好（JSON 格式）
./bin/pibuddy-user set-prefs 小明 '{"style":"简洁直接","interests":["编程","音乐"],"nickname":"程序员","extra":"喜欢用技术解决问题"}'

# 查看用户偏好
./bin/pibuddy-user get-prefs 小明
```

**方式二：语音对话**（仅主人可用）

```bash
# 先设置主人的声纹
./bin/pibuddy-user set-owner 小明
```

然后主人可以通过语音对话设置偏好：
> "你好小派，帮我设置偏好，我喜欢简洁的回复风格，爱好是编程和音乐"

LLM 会调用 `set_user_preferences` 工具完成设置。

### 偏好字段说明

| 字段 | 类型 | 示例 | 说明 |
|------|------|------|------|
| `style` | string | `"简洁直接"` | 回复风格 |
| `interests` | []string | `["编程","音乐"]` | 兴趣爱好 |
| `nickname` | string | `"程序员"` | 昵称 |
| `extra` | string | `"喜欢用技术解决问题"` | 额外描述 |

### 工作原理

每次对话时，系统会：
1. 自动识别说话人声纹
2. 将识别到的用户偏好注入 LLM 的 system prompt
3. LLM 根据偏好调整回复风格和内容

### 配置文件

```yaml
voiceprint:
  enabled: true
  model_path: "./models/voiceprint/3dspeaker_speech_campplus_sv_zh-cn_16k-common.onnx"
  threshold: 0.6
```

### 常用命令

```bash
# 列出所有用户及偏好
./bin/pibuddy-user list

# 设置主人（主人可以语音注册/删除用户）
./bin/pibuddy-user set-owner 小明
```

## 配置说明

配置文件位于 `configs/pibuddy.yaml`：

```yaml
audio:
  sample_rate: 16000      # 采集采样率
  channels: 1             # 单声道
  frame_size: 512         # 每帧样本数

dialog:
  wake_reply: "我在"      # 唤醒回复语
  interrupt_reply: "我在" # 打断回复语
  listen_delay: 500       # 回复后延迟进入监听 (ms)
  continuous_timeout: 15  # 连续对话超时 (秒)

wake:
  model_path: "./models/kws"
  keywords_file: "./models/kws/keywords.txt"
  threshold: 0.5

vad:
  model_path: "./models/vad/silero_vad.onnx"
  threshold: 0.5
  min_silence_ms: 1200    # 静音多久视为说话结束

asr:
  model_path: "./models/asr"
  num_threads: 2

llm:
  # 多模型配置（推荐）
  models:
    - name: "qwen-turbo"
      api_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
      api_key: "${PIBUDDY_QWEN_API_KEY}"
      model: "qwen-turbo"
    # ... 更多模型
  # 兼容旧配置（models 为空时使用）
  provider: "openai"
  api_url: "https://api.deepseek.com/v1"
  api_key: "${PIBUDDY_LLM_API_KEY}"
  model: "deepseek-chat"
  max_history: 10
  max_tokens: 500

tts:
  engine: "tencent"       # tencent | edge | piper
  tencent:
    secret_id: "${PIBUDDY_TENCENT_SECRET_ID}"
    secret_key: "${PIBUDDY_TENCENT_SECRET_KEY}"
    voice_type: 101001    # 晓晓

music:
  enabled: true
  provider: "qqmusic"
  cache_max_size: 500     # 缓存上限 (MB)

voiceprint:
  enabled: true

rss:
  enabled: true
  cache_ttl: 30           # 缓存时间 (分钟)

tools:
  weather_api_key: "${PIBUDDY_WEATHER_API_KEY}"
```

## 工作流程

```
说"你好小派" → 开始录音 → 语音识别 → 发送到大模型 → 语音合成 → 播放回复
     ↑                                              ↓
     └──────────── 说唤醒词可打断 ──────────────────┘
```

**状态机**：`Idle → Listening → Processing → Speaking → Idle`

## 项目结构

```
pibuddy/
├── cmd/
│   ├── main.go               # 主程序入口
│   ├── music/main.go         # 音乐登录工具 (pibuddy-music)
│   └── user/main.go          # 用户管理工具 (pibuddy-user)
├── internal/
│   ├── audio/                # 音频采集、播放、缓存
│   ├── wake/                 # 唤醒词检测
│   ├── vad/                  # 语音端点检测
│   ├── asr/                  # 流式语音识别
│   ├── llm/                  # 大模型对话 + Function Calling
│   ├── tts/                  # 语音合成
│   ├── music/                # 音乐服务客户端
│   ├── rss/                  # RSS 订阅管理
│   ├── voiceprint/           # 声纹识别
│   ├── tools/                # LLM 工具集 (20+ 工具)
│   ├── pipeline/             # 主编排器 + 状态机
│   └── config/               # YAML 配置
├── configs/pibuddy.yaml      # 默认配置
├── scripts/
│   ├── setup.sh              # 树莓派初始化
│   ├── setup-mac.sh          # macOS 测试初始化
│   └── pibuddy.service       # Systemd 服务
├── Makefile                  # 编译与部署
└── README.md
```

## 多 LLM Provider 支持

PiBuddy 支持多 LLM Provider 自动降级，最大化利用免费额度：

| Provider | 免费额度 | 有效期 | 说明 |
|----------|---------|--------|------|
| 通义千问 | 400万 token | 各模型100万，2026/05/22 | qwen-turbo/flash/plus/max |
| 腾讯混元 | 100万 token | 新用户共享 | hunyuan-lite/turbo |
| 火山方舟 | 50万 token | 豆包全系共享 | 需创建接入点 |
| DeepSeek | 付费 | — | 作为最终兜底 |

**总计约 550 万 token 免费额度**，按每次对话 200-500 token 计算，可支持约 11000-27500 次对话。

### 配置示例

```yaml
llm:
  models:
    # 通义千问（快速模型优先）
    - name: "qwen-turbo"
      api_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
      api_key: "${PIBUDDY_QWEN_API_KEY}"
      model: "qwen-turbo"
    - name: "qwen-flash"
      api_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
      api_key: "${PIBUDDY_QWEN_API_KEY}"
      model: "qwen-flash"
    # 腾讯混元
    - name: "hunyuan-lite"
      api_url: "https://api.hunyuan.cloud.tencent.com/v1"
      api_key: "${PIBUDDY_HUNYUAN_API_KEY}"
      model: "hunyuan-lite"
    # 火山方舟
    - name: "doubao"
      api_url: "https://ark.cn-beijing.volces.com/api/v3"
      api_key: "${PIBUDDY_ARK_API_KEY}"
      model: "${PIBUDDY_ARK_ENDPOINT_ID}"
    # DeepSeek（付费兜底）
    - name: "deepseek-chat"
      api_url: "https://api.deepseek.com/v1"
      api_key: "${PIBUDDY_LLM_API_KEY}"
      model: "deepseek-chat"
```

### 兼容其他 OpenAI 协议 API

- **OpenAI**: 直接使用
- **DeepSeek**: `api_url: "https://api.deepseek.com/v1"`, `model: "deepseek-chat"`
- **本地 Ollama**: `api_url: "http://localhost:11434/v1"`, `model: "qwen2.5:7b"`, `api_key: "ollama"`
- **其他兼容服务**: 修改 `api_url` 和 `model` 即可

## 外部依赖

| 依赖 | 用途 | 获取方式 |
|------|------|----------|
| 通义千问 | LLM（推荐，免费额度大） | [阿里云 DashScope](https://dashscope.console.aliyun.com) |
| 腾讯混元 | LLM（免费） | [腾讯云混元](https://console.cloud.tencent.com/hunyuan) |
| 火山方舟 | LLM（豆包，免费） | [火山方舟](https://console.volcengine.com/ark) |
| DeepSeek | LLM（付费兜底） | [DeepSeek 开放平台](https://platform.deepseek.com) |
| 腾讯云 TTS/ASR | 语音合成/识别 | [腾讯云控制台](https://console.cloud.tencent.com/tts) |
| 和风天气 | 天气查询 | [和风天气开发平台](https://dev.qweather.com) |
| QQAFLMusicApi | QQ 音乐服务 | [GitHub](https://github.com/QiuChenlyOpenSource/QQAFLMusicApi) |
| NeteaseCloudMusicApi | 网易云音乐服务 | [GitLab](https://gitlab.com/Binaryify/NeteaseCloudMusicApi) |

## 故障排查

| 问题 | 解决方法 |
|------|---------|
| 没有检测到麦克风 | `arecord -l` 检查设备；确认 ALSA 配置 |
| 唤醒词不灵敏 | 降低 `wake.threshold`（如 0.3） |
| 唤醒词误触发 | 提高 `wake.threshold`（如 0.7） |
| TTS 没声音 | `aplay -l` 检查设备；`speaker-test -c 1` 测试 |
| LLM 无响应 | 检查 API Key 和网络连接 |
| 音乐无法播放 | 检查音乐 API 服务是否运行；使用 `pibuddy-music status` 检查登录状态 |
| 蓝牙音箱没声音 | `pactl list sinks short` 确认蓝牙设备已连接；检查是否设为默认输出 |
| Mac 没有声音 | 系统设置 > 声音 > 确认输出设备正确 |
| Mac 麦克风无法录音 | 系统设置 > 隐私与安全性 > 麦克风 > 允许终端访问 |

## 开发与测试

### 运行单元测试

项目包含纯逻辑单元测试，可在无硬件（无麦克风/扬声器/模型文件）的开发机上运行：

```bash
make test
```

测试覆盖：音频格式转换、LLM 上下文管理、状态机、句子拆分、配置加载、音乐服务、RSS 解析、声纹存储等。

### 构建所有工具

```bash
make build           # 主程序
make build-music     # 音乐登录工具
make build-user      # 用户管理工具
make build-all       # 全部构建
```

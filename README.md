# PiBuddy — 树莓派智能语音助手

把树莓派 4B 变成一个比小爱同学更聪明的智能音箱，用大模型驱动对话。

## 功能

- **语音唤醒**：说"你好小派"唤醒
- **语音识别**：流式中英文 ASR (sherpa-onnx Zipformer)
- **智能对话**：OpenAI 兼容 API，支持任意大模型
- **语音合成**：Edge TTS（在线）或 Piper TTS（离线）
- **打断支持**：播放回复时说唤醒词可打断

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
# 方法 A: 环境变量文件
echo 'PIBUDDY_LLM_API_KEY=sk-your-key-here' > /home/pi/pibuddy/.env

# 方法 B: 直接在配置文件中设置
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

编辑 `configs/pibuddy.yaml`，设置你的 LLM 后端（以 DeepSeek 为例）：

```yaml
llm:
  provider: "openai"
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

## 配置说明

配置文件位于 `configs/pibuddy.yaml`：

```yaml
audio:
  sample_rate: 16000      # 采集采样率
  channels: 1             # 单声道
  frame_size: 512         # 每帧样本数

wake:
  model_path: "./models/kws"
  keywords: "你好小派"
  threshold: 0.5          # 唤醒灵敏度 (0-1, 越低越灵敏)

vad:
  model_path: "./models/vad/silero_vad.onnx"
  threshold: 0.5
  min_silence_ms: 500     # 静音多久视为说话结束

asr:
  model_path: "./models/asr"
  num_threads: 2          # CPU 线程数

llm:
  provider: "openai"
  api_url: "https://api.openai.com/v1"    # 兼容任意 OpenAI 协议的 API
  api_key: "${PIBUDDY_LLM_API_KEY}"       # 从环境变量读取
  model: "gpt-4o-mini"
  system_prompt: "你是小派，一个智能家庭助手。请用简洁友好的中文回答。"
  max_history: 10         # 保留的对话轮数
  max_tokens: 500

tts:
  engine: "edge"          # "edge" (在线) 或 "piper" (离线)
  edge:
    voice: "zh-CN-XiaoxiaoNeural"
  piper:
    model_path: "./models/piper/zh_CN-huayan-medium.onnx"
```

## 工作流程

```
说"你好小派" → 开始录音 → 语音识别 → 发送到大模型 → 语音合成 → 播放回复
                                                          ↑
                                                     说唤醒词可打断
```

**状态机**：`Idle → Listening → Processing → Speaking → Idle`

## 项目结构

```
pibuddy/
├── cmd/pibuddy/main.go           # 程序入口
├── internal/
│   ├── audio/                    # 音频采集与播放 (malgo)
│   ├── wake/                     # 唤醒词检测 (sherpa-onnx KWS)
│   ├── vad/                      # 语音端点检测 (sherpa-onnx Silero VAD)
│   ├── asr/                      # 流式语音识别 (sherpa-onnx Zipformer)
│   ├── llm/                      # 大模型对话 (OpenAI 兼容协议)
│   ├── tts/                      # 语音合成 (Edge TTS / Piper TTS)
│   ├── pipeline/                 # 主编排器 + 状态机
│   └── config/                   # YAML 配置
├── configs/pibuddy.yaml          # 默认配置
├── scripts/
│   ├── setup.sh                  # 树莓派初始化脚本
│   ├── setup-mac.sh              # macOS 测试初始化脚本
│   └── pibuddy.service           # Systemd 服务
├── Makefile                      # 编译与部署
└── README.md
```

## 使用其他 LLM

PiBuddy 兼容所有 OpenAI 协议的 API，包括：

- **OpenAI**: 直接使用
- **DeepSeek**: `api_url: "https://api.deepseek.com/v1"`, `model: "deepseek-chat"`
- **本地 Ollama**: `api_url: "http://localhost:11434/v1"`, `model: "qwen2.5:7b"`, `api_key: "ollama"`
- **其他兼容服务**: 修改 `api_url` 和 `model` 即可

## 故障排查

| 问题 | 解决方法 |
|------|---------|
| 没有检测到麦克风 | `arecord -l` 检查设备；确认 ALSA 配置 |
| 唤醒词不灵敏 | 降低 `wake.threshold`（如 0.3） |
| 唤醒词误触发 | 提高 `wake.threshold`（如 0.7） |
| TTS 没声音 | `aplay -l` 检查设备；`speaker-test -c 1` 测试 |
| LLM 无响应 | 检查 API Key 和网络连接 |
| 蓝牙音箱没声音 | `pactl list sinks short` 确认蓝牙设备已连接；检查是否设为默认输出 |
| Mac 没有声音 | 系统设置 > 声音 > 确认输出设备正确 |
| Mac 麦克风无法录音 | 系统设置 > 隐私与安全性 > 麦克风 > 允许终端访问 |

## 开发与测试

### 运行单元测试

项目包含纯逻辑单元测试，可在无硬件（无麦克风/扬声器/模型文件）的开发机上运行：

```bash
make test
```

测试覆盖范围：

| 测试文件 | 覆盖内容 |
|----------|---------|
| `internal/audio/convert_test.go` | PCM 格式转换：int16/float32/bytes 互转、钳位、往返一致性 |
| `internal/llm/context_test.go` | 对话上下文管理：滑动窗口截断、system 消息前缀、清空 |
| `internal/llm/openai_test.go` | SSE 流式客户端：httptest mock、空 delta 跳过、错误码、context 取消 |
| `internal/pipeline/state_test.go` | 状态机：合法/非法转换、ForceIdle、onChange 回调 |
| `internal/pipeline/sentence_test.go` | 句子拆分：中英文标点、换行符、边界情况 |
| `internal/config/config_test.go` | 配置加载：默认值填充、环境变量展开、YAML 解析 |

测试策略：只用标准库 `testing`，网络依赖用 `net/http/httptest` mock，不引入第三方测试框架。

# Project Context

## Purpose

PiBuddy 是一个运行在树莓派 4B 上的智能语音助手，通过唤醒词激活，流式语音识别用户输入，调用大模型生成回复，再通过 TTS 合成语音播放。目标是把树莓派变成一个由大模型驱动的智能音箱。

## Tech Stack

- **语言**: Go 1.21, CGO enabled
- **音频 I/O**: miniaudio (通过 `gen2brain/malgo` Go 绑定), ALSA 后端
- **语音处理**: sherpa-onnx-go (唤醒词 KWS, VAD Silero, 流式 ASR Zipformer)
- **LLM**: OpenAI 兼容协议 (SSE streaming), 支持任意兼容 API
- **TTS**: Edge TTS (在线, `pp-group/edge-tts-go`) / Piper TTS (离线, CLI 子进程)
- **MP3 解码**: `hajimehoshi/go-mp3` (Edge TTS 输出解码)
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

### Testing Strategy

- 只使用标准库 `testing`, 不引入第三方测试框架
- 测试文件放在对应包目录下 (`_test.go`)
- 纯逻辑组件直接测试 (audio convert, context manager, state machine, sentence split, config)
- 网络依赖用 `net/http/httptest` mock (OpenAI SSE client)
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
- Edge TTS 需要互联网连接; Piper TTS 可离线但需要模型文件

## External Dependencies

| 依赖 | 用途 | 网络要求 |
|------|------|----------|
| OpenAI 兼容 API | 大模型对话 | 需要网络 |
| Edge TTS (Microsoft) | 在线语音合成 | 需要网络 |
| Piper TTS | 离线语音合成 | 不需要 (本地模型) |
| sherpa-onnx models | 唤醒词/VAD/ASR 推理 | 首次下载需要网络 |

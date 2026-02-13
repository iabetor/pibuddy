## 1. 音乐 API 客户端
- [ ] 1.1 新建 `internal/music/provider.go` — 定义 `Provider` 接口（`Search`, `GetStreamURL`）和 `Song` 结构体
- [ ] 1.2 新建 `internal/music/qq.go` — 实现 QQ 音乐 Web 接口客户端（搜索、获取播放 URL）
- [ ] 1.3 新建 `internal/music/qq_test.go` — 用 httptest mock 测试搜索和 URL 获取
- [ ] 1.4 新增 `MusicConfig` 到 `internal/config/config.go`（api_url, cookie, enabled）

## 2. 音频流播放
- [ ] 2.1 新建 `internal/audio/stream.go` — `StreamPlayer` 结构体，支持从 HTTP URL 流式下载 + MP3 解码 + PCM 播放
- [ ] 2.2 新建 `internal/audio/stream_test.go` — 用 httptest 提供静态 MP3 数据测试解码流程
- [ ] 2.3 确认 `go-mp3` 解码后的 PCM 格式与现有 `Player` 采样率兼容，必要时加重采样

## 3. 意图识别与技能分发
- [ ] 3.1 新建 `internal/skill/skill.go` — 定义 `Skill` 接口和 `Router`（意图分发器）
- [ ] 3.2 新建 `internal/skill/music.go` — 音乐技能实现：解析 LLM 回复中的 JSON 意图 → 调用 music.Provider → 播放
- [ ] 3.3 修改 LLM system prompt — 添加意图识别输出格式约定
- [ ] 3.4 新建 `internal/skill/skill_test.go` — 测试意图解析和分发逻辑

## 4. Pipeline 集成
- [ ] 4.1 修改 `internal/pipeline/pipeline.go` — `processQuery` 中 LLM 回复第一句到达后先判断是否为音乐意图
- [ ] 4.2 Pipeline 中集成 `StreamPlayer`，音乐播放走流式路径，TTS 走现有路径
- [ ] 4.3 音乐播放期间复用现有唤醒词打断机制（StateSpeaking + cancelSpeak）
- [ ] 4.4 搜索失败或播放失败时 TTS 语音提示"抱歉，暂时无法播放"

## 5. 配置与文档
- [ ] 5.1 更新 `configs/pibuddy.yaml` — 新增 `music` 配置段
- [ ] 5.2 更新 `README.md` — 新增音乐播放功能说明和配置方法
- [ ] 5.3 补充音乐相关单元测试，确保 `make test` 全部通过

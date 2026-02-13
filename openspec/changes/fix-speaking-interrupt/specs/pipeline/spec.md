## MODIFIED Requirements

### Requirement: 唤醒词打断播放

在 `StateSpeaking` 或 `StateProcessing` 状态下检测到唤醒词时，系统 SHALL 立即停止当前播放和 LLM 响应处理，播放打断回复语（InterruptReply），然后进入监听状态。

打断 SHALL 在整个 processQuery 生命周期内生效，包括：
- 句间 TTS 合成等待期
- LLM 流式响应等待期
- 工具调用执行期
- 音频播放期

#### Scenario: 讲故事时打断
- **WHEN** LLM 正在逐句播放长回复（如讲故事），状态在 Speaking 和 Processing 之间交替
- **AND** 用户在任意时刻说出唤醒词
- **THEN** 系统立即停止播放，processQuery goroutine 快速退出
- **AND** 系统播放 InterruptReply（如"我在"）
- **AND** 系统等待 ListenDelay 毫秒后进入 Listening 状态等待用户输入

#### Scenario: TTS 合成期间打断
- **WHEN** 系统处于 StateProcessing，正在进行下一句的 TTS 网络合成
- **AND** 用户说出唤醒词
- **THEN** 系统设置 interrupted 标志
- **AND** processQuery 在下次检查点检测到标志后退出
- **AND** 系统播放 InterruptReply 并进入 Listening 状态

#### Scenario: 工具调用期间打断
- **WHEN** 系统处于 StateProcessing，正在执行工具调用
- **AND** 用户说出唤醒词
- **THEN** 系统设置 interrupted 标志
- **AND** processQuery 在工具调用循环的检查点退出
- **AND** 系统播放 InterruptReply 并进入 Listening 状态

## ADDED Requirements

### Requirement: 回复语后延迟监听

系统在播放回复语（唤醒回复或打断回复）之后，SHALL 等待可配置的 `ListenDelay` 毫秒（默认 500ms）再进入 Listening 状态，给用户反应时间。

#### Scenario: 唤醒后延迟监听
- **WHEN** 用户说出唤醒词
- **AND** 系统播放完唤醒回复语
- **THEN** 系统等待 ListenDelay 毫秒
- **AND** 然后进入 Listening 状态开始监听

#### Scenario: 打断后延迟监听
- **WHEN** 用户在播放中说出唤醒词打断
- **AND** 系统播放完打断回复语
- **THEN** 系统等待 ListenDelay 毫秒
- **AND** 然后进入 Listening 状态开始监听

### Requirement: interrupted 原子标志

Pipeline SHALL 维护一个 `interrupted` 原子布尔标志，用于跨 goroutine 通信打断信号。

- `processQuery` 入口处 SHALL 重置为 false
- 打断处理函数 SHALL 将其设为 true
- `processQuery` 内所有循环和阻塞点 SHALL 检查此标志

#### Scenario: 标志生命周期
- **WHEN** 新的 processQuery 开始执行
- **THEN** interrupted 标志被重置为 false
- **WHEN** 唤醒词打断触发
- **THEN** interrupted 标志被设为 true
- **AND** processQuery 在下一个检查点退出

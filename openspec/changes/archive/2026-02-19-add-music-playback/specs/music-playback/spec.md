## ADDED Requirements

### Requirement: 语音点歌

用户 SHALL 能够通过语音指令请求播放音乐，PiBuddy 通过 LLM 识别音乐播放意图并提取歌曲关键词。

#### Scenario: 用户说出明确歌名
- **WHEN** 用户说"播放小星星"
- **THEN** LLM 识别意图为播放音乐，提取关键词"小星星"
- **THEN** 系统搜索歌曲并开始播放

#### Scenario: 用户说出模糊请求
- **WHEN** 用户说"来首周杰伦的歌"
- **THEN** LLM 识别意图为播放音乐，提取关键词"周杰伦"
- **THEN** 系统搜索并播放第一个匹配结果

#### Scenario: 非音乐请求不受影响
- **WHEN** 用户说"今天天气怎么样"
- **THEN** LLM 识别意图为普通对话
- **THEN** 系统走现有 TTS 回复流程，行为不变

### Requirement: 音乐搜索与流式播放

系统 SHALL 通过音乐 API 搜索歌曲、获取播放 URL，并以流式方式解码和播放音频。

#### Scenario: 搜索并播放成功
- **WHEN** 音乐 API 返回有效搜索结果和播放 URL
- **THEN** 系统先 TTS 播报"正在为你播放 xxx"
- **THEN** 从 URL 流式下载 MP3，解码为 PCM，通过扬声器播放

#### Scenario: 搜索无结果
- **WHEN** 音乐 API 搜索返回空结果
- **THEN** 系统 TTS 回复"没有找到这首歌"

#### Scenario: API 不可用
- **WHEN** 音乐 API 请求失败（网络错误、接口变更等）
- **THEN** 系统 TTS 回复"音乐服务暂时不可用"
- **THEN** 系统回到 Idle 状态

### Requirement: 音乐播放打断

音乐播放期间 SHALL 支持唤醒词打断，复用现有打断机制。

#### Scenario: 播放中唤醒词打断
- **WHEN** 正在播放音乐
- **WHEN** 用户说出唤醒词
- **THEN** 音乐播放立即停止
- **THEN** 系统进入 Listening 状态，准备接收新指令

### Requirement: 音乐功能配置

音乐功能 SHALL 通过配置文件启用和配置。

#### Scenario: 配置启用音乐
- **WHEN** 配置文件中 `music.enabled` 为 `true` 且 `music.api_url` 已设置
- **THEN** 系统启动时初始化音乐 Provider

#### Scenario: 未配置时降级
- **WHEN** 配置文件中 `music.enabled` 为 `false` 或未设置
- **THEN** 音乐相关语音请求走普通对话流程（LLM 文字回复）

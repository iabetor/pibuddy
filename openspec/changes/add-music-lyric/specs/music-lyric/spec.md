## ADDED Requirements

### Requirement: 歌词获取

系统 SHALL 支持从音乐平台获取歌曲歌词。

#### Scenario: QQ 音乐获取歌词成功
- **WHEN** 用户播放一首有歌词的歌曲
- **AND** 音乐来源为 QQ 音乐
- **THEN** 系统调用 QQ 音乐歌词接口获取歌词
- **AND** 解析 LRC 格式返回 Lyric 对象

#### Scenario: 网易云音乐获取歌词成功
- **WHEN** 用户播放一首有歌词的歌曲
- **AND** 音乐来源为网易云音乐
- **THEN** 系统调用网易云音乐歌词接口获取歌词
- **AND** 解析 JSON 格式返回 Lyric 对象

#### Scenario: 歌曲无歌词
- **WHEN** 用户播放一首没有歌词的歌曲
- **THEN** 系统返回空歌词
- **AND** 不显示歌词内容

### Requirement: LRC 格式解析

系统 SHALL 支持解析标准 LRC 格式歌词。

#### Scenario: 解析标准 LRC 格式
- **GIVEN** LRC 内容包含元数据和时间标签
- **WHEN** 系统解析 LRC 内容
- **THEN** 提取歌曲名、歌手、专辑等元数据
- **AND** 提取所有时间标签和对应歌词文本
- **AND** 按时间戳排序返回

#### Scenario: 解析多时间标签行
- **GIVEN** LRC 行包含多个时间标签如 `[00:15.50][01:30.20]歌词`
- **WHEN** 系统解析该行
- **THEN** 生成多条 LyricLine，每条对应一个时间标签

### Requirement: 歌词缓存

系统 SHALL 缓存已获取的歌词以减少 API 调用。

#### Scenario: 缓存命中
- **GIVEN** 某歌曲歌词已缓存
- **WHEN** 再次请求该歌曲歌词
- **THEN** 直接从缓存返回，不调用 API

#### Scenario: 缓存未命中
- **GIVEN** 某歌曲歌词未缓存
- **WHEN** 请求该歌曲歌词
- **THEN** 调用 API 获取歌词
- **AND** 将歌词存入缓存

### Requirement: 歌词 API

系统 SHALL 提供 HTTP API 供前端获取当前歌词。

#### Scenario: 获取当前歌词
- **WHEN** 前端请求 `/api/lyric/current`
- **THEN** 返回当前播放歌曲的歌词
- **AND** 包含当前播放进度对应的歌词行

#### Scenario: 未播放歌曲时请求歌词
- **WHEN** 前端请求 `/api/lyric/current`
- **AND** 当前没有播放任何歌曲
- **THEN** 返回空响应或错误提示

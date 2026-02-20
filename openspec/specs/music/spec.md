# music Specification

## Purpose
TBD - created by archiving change add-music-resume. Update Purpose after archive.
## Requirements
### Requirement: Music Pause on Interrupt

The system SHALL pause music playback instead of stopping it when the wake word is detected during playback, preserving the playback state for later resumption.

#### Scenario: Wake word interrupts music
- **WHEN** music is playing and user says the wake word
- **THEN** the system pauses the music and preserves the playlist, current index, and play mode

#### Scenario: Non-wake-word interruption
- **WHEN** user explicitly says "停止播放" or "暂停音乐"
- **THEN** the system stops the music without preserving state (no resume available)

### Requirement: Resume Music Playback

The system SHALL provide a tool to resume paused music from the interrupted point.

#### Scenario: Resume after interrupt
- **WHEN** user says "继续播放" or "恢复播放" after an interrupt
- **THEN** the system resumes the paused music from the saved state

#### Scenario: Resume with expired URL
- **WHEN** user tries to resume but the song URL has expired
- **THEN** the system refreshes the URL and continues playing

#### Scenario: Resume with no paused music
- **WHEN** user says "继续播放" but no music was paused
- **THEN** the system responds "没有暂停的音乐"

#### Scenario: Resume after playlist ended
- **WHEN** user says "继续播放" but the previous playlist had finished
- **THEN** the system responds "播放列表已结束"

### Requirement: Preserve Playlist State

The system SHALL preserve the complete playlist state when music is paused by interrupt.

#### Scenario: Preserve playlist and position
- **WHEN** music is interrupted during the 3rd song in a 5-song playlist
- **THEN** the system saves all 5 songs, the current index (2), and the play mode

#### Scenario: Preserve play mode
- **WHEN** music is interrupted while in loop mode
- **THEN** the system saves the loop mode and resumes with the same mode

### Requirement: Clear Pause State on Explicit Stop

The system SHALL clear the pause state when user explicitly requests to stop or pause music.

#### Scenario: Explicit stop clears state
- **WHEN** user says "停止播放" or "暂停音乐"
- **THEN** the system clears the pause state and "继续播放" will not work

#### Scenario: New music replaces pause state
- **WHEN** user plays new music after an interrupt
- **THEN** the old pause state is replaced by the new playback

### Requirement: Music Favorites Storage

The system SHALL provide persistent storage for user favorite songs, with separate playlists for each identified user.

#### Scenario: Add song to favorites
- **WHEN** user says "收藏这首歌" or "我喜欢这首歌" while a song is playing
- **THEN** the system saves the current song to the user's favorite list

#### Scenario: Add to default favorites for unidentified user
- **WHEN** voice recognition fails to identify the user
- **THEN** the song is saved to the default "guest" favorites list

#### Scenario: Favorites persist after restart
- **WHEN** the system restarts
- **THEN** all previously saved favorites remain available

### Requirement: Play Favorites

The system SHALL allow users to play their saved favorite songs with random or sequential playback mode.

#### Scenario: Play favorites randomly
- **WHEN** user says "播放我的收藏" or "播放我喜欢的歌"
- **THEN** the system plays all saved favorites in random order

#### Scenario: Play favorites sequentially
- **WHEN** user says "顺序播放我的收藏"
- **THEN** the system plays favorites in the order they were added

#### Scenario: Empty favorites
- **WHEN** user tries to play favorites but the list is empty
- **THEN** the system responds "你的收藏是空的，先收藏一些歌曲吧"

### Requirement: List Favorites

The system SHALL allow users to view their saved favorite songs.

#### Scenario: List favorites
- **WHEN** user says "我收藏了哪些歌" or "我的收藏有什么"
- **THEN** the system lists all saved favorite songs with names and artists

#### Scenario: Empty favorites list
- **WHEN** user asks for favorites but has none saved
- **THEN** the system responds "你还没有收藏任何歌曲"

### Requirement: Remove from Favorites

The system SHALL allow users to remove songs from their favorites.

#### Scenario: Remove current song from favorites
- **WHEN** user says "把这首歌从收藏删掉" while a song is playing
- **THEN** the system removes the current song from favorites

#### Scenario: Remove non-existent song
- **WHEN** user tries to remove a song not in favorites
- **THEN** the system responds "这首歌不在你的收藏中"

### Requirement: Multi-user Favorites

The system SHALL maintain separate favorites lists for different users identified by voice recognition.

#### Scenario: Different users have separate favorites
- **WHEN** user A adds a song to favorites
- **AND** user B asks to play favorites
- **THEN** user B's favorites are played, not user A's

#### Scenario: Unidentified user uses default list
- **WHEN** voice recognition cannot identify the speaker
- **THEN** the default "guest" favorites list is used

### Requirement: 音乐登录管理命令

系统 SHALL 提供 `pibuddy-music` 命令行工具，用于管理网易云音乐登录状态。

#### Scenario: 用户登录网易云音乐

- **WHEN** 用户执行 `pibuddy-music login`
- **THEN** 系统提示用户在浏览器打开 NeteaseCloudMusicApi 登录页面
- **AND** 用户完成登录后，系统从 API 获取 cookie 并保存到本地文件

#### Scenario: 查询登录状态

- **WHEN** 用户执行 `pibuddy-music status`
- **THEN** 系统显示本地保存的登录信息和 API 实时登录状态
- **AND** 如果 cookie 已过期，提示用户重新登录

#### Scenario: 退出登录

- **WHEN** 用户执行 `pibuddy-music logout`
- **THEN** 系统删除本地保存的 cookie 文件

### Requirement: 自动加载登录 Cookie

NeteaseClient SHALL 自动从本地文件加载登录 cookie 并附加到所有 HTTP 请求。

#### Scenario: 播放音乐时自动使用 cookie

- **WHEN** 用户请求播放音乐
- **THEN** NeteaseClient 自动从 `~/.pibuddy/netease_cookie.json` 加载 cookie
- **AND** 将 cookie 附加到搜索和获取播放地址的 HTTP 请求

#### Scenario: Cookie 缓存优化

- **WHEN** NeteaseClient 需要发送 HTTP 请求
- **THEN** cookie 文件最多每分钟读取一次（缓存机制）
- **AND** 避免频繁的磁盘 IO 操作


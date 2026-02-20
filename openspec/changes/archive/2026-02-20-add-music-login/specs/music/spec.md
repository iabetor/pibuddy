## ADDED Requirements

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

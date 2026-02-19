# Change: 网易云音乐登录管理

## Why

网易云音乐 API 需要登录才能获取完整的歌曲播放地址，但登录 cookie 会定期过期，需要用户手动更新。当前用户需要手动操作 NeteaseCloudMusicApi 的 cookie，体验不佳：

1. **Cookie 过期问题**：登录状态失效后无法播放完整歌曲
2. **操作繁琐**：需要手动操作 NeteaseCloudMusicApi 服务来刷新 cookie
3. **无状态提示**：用户不知道当前登录状态是否有效

## What Changes

- 新增 `pibuddy-music` 命令行工具，支持浏览器登录并自动保存 cookie
- `NeteaseClient` 自动从本地文件加载 cookie 并附加到请求
- 支持查看登录状态和退出登录

### 命令行用法

```bash
# 登录（提示在浏览器打开登录页面）
pibuddy-music login

# 查看登录状态
pibuddy-music status

# 退出登录
pibuddy-music logout
```

### 登录流程

1. 用户运行 `pibuddy-music login`
2. 程序提示在浏览器打开 NeteaseCloudMusicApi 登录页
3. 用户扫码或账号登录后按回车
4. 程序从 API 获取 cookie 并保存到 `~/.pibuddy/netease_cookie.json`
5. 后续请求自动携带 cookie

## Impact

- **新增文件**：
  - `cmd/music/main.go` — 登录命令行工具
- **修改文件**：
  - `internal/music/netease.go` — 添加 cookie 自动加载逻辑
  - `internal/pipeline/pipeline.go` — 使用新的 NeteaseClient 构造函数
  - `Makefile` — 添加 `build-music` 目标
- **数据文件**：
  - `~/.pibuddy/netease_cookie.json` — 存储登录 cookie

## Success Criteria

- [ ] `pibuddy-music login` 能完成登录并保存 cookie
- [ ] `pibuddy-music status` 能正确显示登录状态
- [ ] `pibuddy-music logout` 能清除本地 cookie
- [ ] 主程序播放音乐时自动使用已保存的 cookie
- [ ] cookie 过期后重新登录即可刷新

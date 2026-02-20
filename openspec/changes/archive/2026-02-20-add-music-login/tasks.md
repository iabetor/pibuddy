## 1. 实现登录命令行工具

- [x] 1.1 创建 `cmd/music/main.go`
- [x] 1.2 实现 `login` 子命令：提示浏览器登录，获取并保存 cookie
- [x] 1.3 实现 `status` 子命令：显示本地和 API 登录状态
- [x] 1.4 实现 `logout` 子命令：清除本地 cookie 文件

## 2. 修改 NeteaseClient 支持自动加载 cookie

- [x] 2.1 添加 `NewNeteaseClientWithDataDir` 构造函数
- [x] 2.2 实现 cookie 文件读取（带缓存，每分钟最多读一次）
- [x] 2.3 在 HTTP 请求中自动附加 Cookie 头
- [x] 2.4 更新 `Search` 和 `GetSongURL` 使用新的请求方法

## 3. 集成到主程序

- [x] 3.1 修改 `pipeline.go` 使用 `NewNeteaseClientWithDataDir`
- [x] 3.2 添加 Makefile `build-music` 目标

## 4. 测试验证

- [ ] 4.1 测试 `pibuddy-music login` 登录流程
- [ ] 4.2 测试 `pibuddy-music status` 状态查询
- [ ] 4.3 测试主程序播放音乐时 cookie 自动加载
- [ ] 4.4 测试 cookie 过期后重新登录刷新

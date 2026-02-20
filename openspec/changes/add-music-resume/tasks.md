## 1. 播放器层

- [x] 1.1 更新 `internal/audio/stream.go`
  - [x] Stop() 方法已支持暂停功能（cancel 保留上下文）
  - 注：精确暂停位置需要更大改动，当前方案是保存播放列表状态

## 2. 管道层

- [x] 2.1 更新 `internal/pipeline/pipeline.go`
  - [x] 添加 `pausedMusic` 字段保存暂停的音乐信息
  - [x] 添加 `savePausedMusic()` 方法
  - [x] 修改 `interruptSpeak()` - 保存播放状态
  - [x] 添加 `resumeMusicCh` channel 用于恢复播放
  - [x] 修改 `Run()` - 监听恢复播放信号
  - [x] 添加 `resumeMusicPlayback()` 方法

## 3. 播放列表层

- [x] 3.1 更新 `internal/music/playlist.go`
  - [x] 添加 `GetItems()` 方法获取播放列表副本

## 4. 工具层

- [x] 4.1 创建 `internal/tools/music_resume.go`
  - [x] ResumeMusicTool - 恢复暂停的音乐
    - [x] 检查是否有暂停的音乐
    - [x] 恢复播放列表和播放模式
    - [x] 通过 channel 通知 Pipeline 开始播放
  - [x] StopMusicTool - 停止播放（清除暂停状态）

## 5. 工具注册

- [x] 5.1 更新 `internal/pipeline/pipeline.go`
  - [x] 注册 ResumeMusicTool
  - [x] 注册 StopMusicTool

## 6. 从指定位置恢复播放

- [x] 6.1 更新 `internal/pipeline/pipeline.go`
  - [x] 添加 `musicPlayStart` 字段记录播放开始时间
  - [x] 添加 `currentCacheKey` 字段记录当前歌曲缓存 key
  - [x] 在 `playMusicFromPosition` 中记录播放开始时间和 cacheKey
  - [x] 在 `savePausedMusic` 中计算并保存播放位置

- [x] 6.2 更新 `internal/audio/stream.go`
  - [x] 添加 `PlayFromPosition(filePath string, positionSec float64)` 方法
  - [x] 实现跳过指定秒数的样本后再播放

- [x] 6.3 更新 `internal/tools/music_resume.go`
  - [x] 修改 `ResumeMusicTool.Execute` 返回位置信息
  - [x] 检查暂停时间是否超过1分钟
  - [x] 检查缓存是否存在

- [x] 6.4 更新 `internal/tools/music.go`
  - [x] 在 `MusicResult` 中添加 `PositionSec` 字段

- [x] 6.5 更新 `internal/pipeline/pipeline.go`
  - [x] 修改 `processQuery` 解析 `resume_music` 结果时传递 `PositionSec`
  - [x] 添加 `playMusicFromPosition` 方法
  - [x] 从缓存文件位置播放失败时回退到从头播放

## 7. 测试

- [ ] 7.1 单元测试
  - [ ] 暂停状态管理
  - [ ] 位置计算逻辑
- [ ] 7.2 集成测试
  - [ ] 唤醒打断 → 恢复播放流程
  - [ ] 多次打断恢复
  - [ ] 超时后从头播放
  - [ ] 从指定位置恢复播放

## 已完成

基础暂停/恢复功能已实现！

### 文件变更

```
internal/audio/stream.go         # 无需修改（Stop 已支持）
internal/music/playlist.go       # 添加 GetItems()
internal/tools/music_resume.go   # 新增
internal/pipeline/pipeline.go    # 修改 - 暂停状态、恢复播放
```

### 功能列表

| 工具 | 说明 |
|------|------|
| resume_music | 恢复暂停的音乐 |
| stop_music | 停止播放（清除暂停状态） |

### 行为

| 场景 | 行为 |
|------|------|
| 唤醒打断 | 保存播放状态 |
| "继续播放" | 恢复暂停的歌曲 |
| "停止播放" | 清除暂停状态 |

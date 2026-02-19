# 实现任务清单

## 倒计时器 (Timer)

### 基础设施
- [ ] 创建 `internal/tools/timer.go`
- [ ] 实现 `TimerStore` 结构体（存储、持久化）
- [ ] 实现 `TimerEntry` 结构体（ID、时长、剩余时间、标签、创建时间）

### 工具实现
- [ ] 实现 `SetTimerTool` - 设置倒计时
- [ ] 实现 `ListTimersTool` - 查看倒计时列表
- [ ] 实现 `CancelTimerTool` - 取消倒计时
- [ ] 实现 `GetRemainingTimeTool` - 查询剩余时间

### 后台管理
- [ ] 实现 `TimerManager` - 管理倒计时生命周期
- [ ] 添加到期通知 channel，与 pipeline 集成
- [ ] 在 pipeline 中处理倒计时到期事件

### 测试
- [ ] 编写 `timer_test.go` 单元测试
- [ ] 测试边界情况（0秒、负数、超大值）

---

## 音量控制 (Volume Control)

### 基础设施
- [ ] 创建 `internal/tools/volume.go`
- [ ] 实现 `VolumeController` 接口
- [ ] 实现 Linux (PulseAudio) 音量控制
- [ ] 实现 Linux (ALSA) 回退方案
- [ ] 实现 macOS 音量控制（开发测试用）

### 工具实现
- [ ] 实现 `SetVolumeTool` - 设置绝对音量
- [ ] 实现 `AdjustVolumeTool` - 相对调节音量（±10）
- [ ] 实现 `MuteTool` - 静音/取消静音
- [ ] 实现 `GetVolumeTool` - 查询当前音量

### 集成
- [ ] 在 pipeline 初始化时创建 VolumeController
- [ ] 音量状态持久化到配置目录

### 测试
- [ ] 编写 `volume_test.go` 单元测试
- [ ] 在树莓派上测试 PulseAudio 和 ALSA 兼容性

---

## 工具注册

- [ ] 在 `internal/pipeline/pipeline.go` 中注册新工具
- [ ] 更新启动日志显示已注册工具数量

---

## 文档更新

- [ ] 更新 `README.md` 功能列表
- [ ] 更新 `openspec/project.md` 工具列表

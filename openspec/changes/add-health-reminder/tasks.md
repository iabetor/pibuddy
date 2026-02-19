# 实现任务清单

## 状态说明
- [ ] 待开始
- [~] 进行中
- [x] 已完成

---

## Phase 1: 基础结构

- [ ] 创建 `internal/tools/health_store.go`
  - [ ] 定义 HealthReminder 结构体
  - [ ] 定义 HealthStore 结构体
  - [ ] 实现 NewHealthStore()
  - [ ] 实现 load() 和 save() 方法
  - [ ] 实现 isQuietHours() 方法

- [ ] 更新 `internal/config/config.go`
  - [ ] 添加 HealthConfig 结构体
  - [ ] 添加 QuietHoursConfig 结构体
  - [ ] 设置默认值

---

## Phase 2: 工具实现

- [ ] 创建 `internal/tools/health.go`
  - [ ] 实现 SetHealthReminderTool
    - [ ] 开启喝水提醒
    - [ ] 开启久坐提醒
    - [ ] 开启吃药提醒
    - [ ] 关闭提醒
    - [ ] 查询状态
  - [ ] 实现 ListHealthRemindersTool
  - [ ] 实现提醒文案（随机选择）

- [ ] 实现 HealthStore 方法
  - [ ] SetReminder() - 设置提醒
  - [ ] GetReminder() - 获取单个提醒
  - [ ] ListReminders() - 列出所有提醒
  - [ ] DeleteReminder() - 删除提醒
  - [ ] CheckAndTrigger() - 检查并触发提醒

---

## Phase 3: Pipeline 集成

- [ ] 更新 `internal/pipeline/pipeline.go`
  - [ ] 添加 healthStore 字段
  - [ ] 初始化 HealthStore
  - [ ] 注册健康提醒工具
  - [ ] 实现 healthReminderChecker() 定时检查
  - [ ] 在 Run() 中启动检查器

---

## Phase 4: 配置更新

- [ ] 更新 `configs/pibuddy.yaml`
  - [ ] 添加 health 配置节
  - [ ] 配置默认间隔
  - [ ] 配置静音时段

- [ ] 更新系统提示词
  - [ ] 添加健康提醒能力说明
  - [ ] 添加使用示例

---

## Phase 5: 测试

- [ ] 创建 `internal/tools/health_test.go`
  - [ ] 测试开启/关闭提醒
  - [ ] 测试吃药提醒时间点
  - [ ] 测试静音时段检测
  - [ ] 测试间隔计算
  - [ ] 测试存储持久化

---

## Phase 6: 文档和验收

- [ ] 运行完整测试
- [ ] 手动测试所有功能
- [ ] 更新 proposal 标记为完成

---

## 功能验收清单

| 功能 | 测试命令 | 预期结果 |
|------|---------|---------|
| 开启喝水提醒 | "提醒我每两小时喝水" | 已开启喝水提醒，每2小时提醒一次 |
| 开启久坐提醒 | "开启久坐提醒" | 已开启久坐提醒，每1小时提醒一次 |
| 开启吃药提醒 | "每天早8点和晚8点提醒我吃降压药" | 已设置降压药提醒，每天08:00和20:00 |
| 关闭提醒 | "关闭喝水提醒" | 喝水提醒已关闭 |
| 查询状态 | "健康提醒状态" | 列出所有提醒状态 |
| 静音时段 | 23:00 后触发提醒 | 不播报，但更新提醒时间 |

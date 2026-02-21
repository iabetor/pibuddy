# 任务清单

## 阶段 1：配置结构扩展 ✅ 已完成

### 1.1 配置定义
- [x] 新增 `LLMModelConfig` 结构体
  - `Name` - 模型名称标识
  - `APIURL` - API 端点地址
  - `APIKey` - 认证密钥
  - `Model` - 模型标识符
- [x] 扩展 `LLMConfig` 支持 `models` 列表
- [x] 向后兼容旧配置格式

---

## 阶段 2：MultiProvider 实现 ✅ 已完成

### 2.1 核心实现
- [x] `MultiProvider` 结构体
  - `providers` - 按优先级排列的 provider 列表
  - `current` - 当前活跃索引
  - `mu` - 读写锁
- [x] `ChatStreamWithTools` 方法
  - 从当前索引开始尝试
  - 失败时自动切换到下一个
  - 记录切换日志
- [x] 额度耗尽检测
  - HTTP 402（DeepSeek 余额不足）
  - HTTP 429（速率限制/额度耗尽）
  - 响应体关键词检测

### 2.2 Provider 集成
- [x] 通义千问（Qwen）
  - qwen-turbo、qwen-flash、qwen-plus、qwen-max
- [x] 腾讯混元（Hunyuan）
  - hunyuan-lite、hunyuan-turbo
- [x] 火山方舟（豆包）
  - 使用接入点 ID 作为 model
- [x] DeepSeek
  - 作为付费兜底

---

## 阶段 3：Pipeline 集成 ✅ 已完成

- [x] 检测 `models` 列表长度
- [x] 多模型时创建 `MultiProvider`
- [x] 单模型时使用 `OpenAIProvider`
- [x] 空列表时使用旧配置（向后兼容）

---

## 阶段 4：Bug 修复 ✅ 已完成

- [x] 修复 SkipLLM 流程后的状态问题
  - 故事播放完成后正确进入连续模式
- [x] 修复中断后重复播放问题
  - 使用 `queryCtx` 而非 `ctx` 控制播放

---

## 完成状态汇总

| 功能 | 状态 |
|------|------|
| 配置结构扩展 | ✅ |
| MultiProvider 实现 | ✅ |
| 顺序降级策略 | ✅ |
| 额度耗尽检测 | ✅ |
| 通义千问集成 | ✅ |
| 腾讯混元集成 | ✅ |
| 火山方舟集成 | ✅ |
| DeepSeek 兜底 | ✅ |
| Pipeline 集成 | ✅ |
| Bug 修复 | ✅ |

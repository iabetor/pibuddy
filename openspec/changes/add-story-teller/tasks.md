# 任务清单

## 阶段 1：本地故事库 ✅ 已完成

### 1.1 故事数据
- [x] 创建内置故事 JSON 文件
  - `models/stories/fairy_tales.json` - 童话故事
  - `models/stories/fables.json` - 寓言故事
  - `models/stories/chinese_stories.json` - 中国传统故事
  - `models/stories/idiom_stories.json` - 成语故事
  - `models/stories/bedtime_stories.json` - 睡前故事
  - 共计 58 个故事

### 1.2 数据存储
- [x] 创建 SQLite stories 表
  - 支持故事 ID、标题、分类、标签、内容
  - 支持播放计数、评分、来源追踪
  - 支持自动导入 JSON 文件
- [x] 实现 StoryStore
  - 使用统一数据库 `*database.DB`
  - 支持故事查找（标题、分类、标签）
  - 支持保存故事（LLM 生成的可持久化）

### 1.3 工具实现
- [x] TellStoryTool - 讲故事工具
- [x] ListStoriesTool - 列出故事工具
- [x] SaveStoryTool - 保存故事工具（用户可保存喜欢的故事）
- [x] DeleteStoryTool - 删除故事工具

### 1.4 配置与集成
- [x] 配置支持（StoryConfig）
- [x] Pipeline 集成
- [x] System prompt 更新

---

## 阶段 2：外部 API 扩展 ✅ 已完成

- [x] StoryAPI 实现
  - mxnzp 故事 API 客户端
  - 故事分类列表 (`/api/story/types`)
  - 故事列表 (`/api/story/list`)
  - 故事搜索 (`/api/story/search`)
  - 故事详情 (`/api/story/details`)
- [x] API 认证（app_id, app_secret）
- [x] QPS 限制处理（1.1秒延迟）
- [x] API 结果缓存到本地 SQLite

---

## 阶段 3：LLM 绕过机制 ✅ 已完成

- [x] StoryResult 结构体
  - `Success` - 是否成功
  - `Content` - 故事内容
  - `SkipLLM` - 是否跳过 LLM
- [x] TellStoryTool 返回 StoryResult
- [x] Pipeline 检测 SkipLLM 标记
- [x] 直接送 TTS 逻辑
- [x] output_mode 配置项
  - `raw` - 原文朗读，零 token
  - `summarize` - LLM 总结后朗读

---

## 阶段 4：删除故事功能 ✅ 已完成

- [x] StoryStore 删除方法
  - `DeleteStory(id)` - 按 ID 删除
  - `DeleteByKeyword(keyword)` - 按关键词删除
  - `DeleteAllUserStories()` - 删除所有用户故事
  - `DeleteAllCachedStories()` - 清空 API 缓存
- [x] DeleteStoryTool 实现
  - 支持关键词删除
  - 支持批量删除用户故事
  - 支持清空 API 缓存
- [x] 安全限制：不能删除内置故事（source=local）

---

## 阶段 5：测试与文档（待实现）

- [ ] 单元测试
  - StoryStore 测试
  - TellStoryTool 测试
- [ ] 集成测试
- [ ] 使用文档

---

## 完成状态汇总

| 功能 | 状态 |
|------|------|
| 内置故事数据 | ✅ |
| SQLite 存储 | ✅ |
| 故事查找 | ✅ |
| 故事保存 | ✅ |
| 故事删除 | ✅ |
| 播放计数 | ✅ |
| 外部 API | ✅ |
| LLM 兜底提示 | ✅ |
| LLM 绕过机制 | ✅ |
| 输出模式配置 | ✅ |

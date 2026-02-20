## 1. 基础设施

- [x] 1.1 添加 go-pinyin 依赖到 go.mod
- [x] 1.2 更新 config.go 添加 learning 配置结构
- [x] 1.3 更新 pibuddy.yaml 添加默认配置

## 2. 汉字拼音功能

- [x] 2.1 创建 `internal/tools/pinyin.go`
  - [x] PinyinTool 结构体和接口实现
  - [x] 汉字转拼音（支持多音字）
  - [x] 带声调/无声调模式
  - [x] 生僻字查询（可选：集成字典解释）

## 3. 英语学习功能

- [x] 3.1 创建 `internal/tools/english.go`
  - [x] EnglishWordTool - 单词查询（有道词典 API）
    - [x] 查询单词释义
    - [x] 返回发音（音标 + 音频 URL）
    - [x] 返回例句
  - [x] EnglishDailyTool - 每日一句（金山词霸 API）
    - [x] 获取每日一句
    - [x] 返回英文 + 中文 + 图片

- [x] 3.2 创建 `internal/tools/vocabulary_store.go`
  - [x] VocabularyStore - 生词本存储
  - [x] AddWord() - 添加生词
  - [x] ListWords() - 列出生词
  - [x] RemoveWord() - 删除生词
  - [x] VocabularyTool - 生词本工具接口

- [x] 3.3 创建 `internal/tools/english_quiz.go`
  - [x] EnglishQuizTool - 单词测验
  - [x] 内置高频词库（四六级/雅思常见词）
  - [x] 随机出题
  - [x] 判断答案正确性

## 4. 古诗词功能

- [x] 4.1 创建 `internal/tools/poetry.go`
  - [x] PoetryClient - API 客户端封装
  - [x] PoetryDailyTool - 每日一诗
    - [x] 获取推荐诗词
    - [x] 返回标题、作者、正文、译文
  - [x] PoetrySearchTool - 诗词搜索
    - [x] 按关键词搜索
    - [x] 按作者搜索
    - [x] 按名句搜索
    - [x] 下一句查询

- [x] 4.2 创建 `internal/tools/poetry_game.go`
  - [x] PoetryGameTool - 诗词接龙/飞花令
    - [x] 飞花令：给定关键字，轮流背诵含该字的诗句
    - [x] 诗词接龙：上句结尾字作为下句开头
    - [x] 游戏状态管理（当前轮次、历史记录）
    - [x] AI 自动回复诗句

- [ ] 4.3 创建 `internal/tools/poetry_cache.go`（可选）
  - [ ] 诗词本地缓存
  - [ ] 减少 API 调用

## 5. 工具注册

- [x] 5.1 更新 `internal/pipeline/pipeline.go`
  - [x] 注册所有学习工具
  - [x] 条件注册（根据配置启用）

## 6. 测试与文档

- [ ] 6.1 单元测试
- [ ] 6.2 集成测试（语音交互测试）
- [ ] 6.3 更新 README 或用户文档

## 实现顺序建议

1. **拼音功能** - 最简单，本地库无需 API ✅
2. **每日一句** - 简单 API，快速验证 ✅
3. **单词查询** - 核心 API 功能 ✅
4. **生词本** - 本地存储，依赖单词查询 ✅
5. **单词测验** - 依赖词库和生词本 ✅
6. **每日一诗** - API 调用 ✅
7. **诗词搜索** - API 调用 ✅
8. **飞花令/接龙** - 最复杂，需要游戏逻辑 ✅

## 已完成

核心功能已全部实现！可选优化：
- 诗词本地缓存（减少 API 调用）
- 单元测试
- 集成测试

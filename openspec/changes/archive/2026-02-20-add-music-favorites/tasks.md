## 1. 存储层

- [x] 1.1 创建 `internal/music/favorites.go`
  - [x] FavoritesStore 结构体
  - [x] Add() - 添加收藏
  - [x] Remove() - 删除收藏
  - [x] List() - 列出收藏
  - [x] 文件读写和持久化
  - [x] 默认歌单（guest）处理

## 2. 工具层

- [x] 2.1 创建 `internal/tools/music_favorites.go`
  - [x] AddFavoriteTool - 收藏当前播放歌曲
    - [x] 获取当前播放歌曲信息
    - [x] 获取当前用户（从 ContextManager）
    - [x] 调用 FavoritesStore.Add()
  - [x] PlayFavoritesTool - 播放收藏
    - [x] 获取当前用户
    - [x] 加载收藏列表
    - [x] 随机/顺序播放模式
    - [x] 清空当前播放列表，填充收藏歌曲
  - [x] ListFavoritesTool - 列出收藏
    - [x] 获取当前用户
    - [x] 格式化输出收藏列表
  - [x] RemoveFavoriteTool - 删除收藏
    - [x] 获取当前用户
    - [x] 获取当前播放歌曲ID
    - [x] 调用 FavoritesStore.Remove()

## 3. 工具注册

- [x] 3.1 更新 `internal/pipeline/pipeline.go`
  - [x] 创建 FavoritesStore 实例
  - [x] 注册所有收藏相关工具
  - [x] 传递 ContextManager 给工具

## 4. 测试

- [ ] 4.1 单元测试
  - [ ] FavoritesStore 的增删查改
- [ ] 4.2 集成测试
  - [ ] 声纹识别 + 收藏流程
  - [ ] 播放收藏歌曲

## 已完成

核心功能已全部实现！

### 文件变更

```
internal/music/favorites.go      # 新增 - 收藏存储
internal/tools/music_favorites.go # 新增 - 收藏工具
internal/pipeline/pipeline.go    # 修改 - 注册工具
```

### 功能列表

| 工具 | 说明 |
|------|------|
| add_favorite | 收藏当前播放歌曲 |
| remove_favorite | 删除收藏 |
| list_favorites | 列出收藏 |
| play_favorites | 播放收藏（随机/顺序） |

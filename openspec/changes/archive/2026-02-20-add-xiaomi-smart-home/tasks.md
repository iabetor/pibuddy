# 实现任务清单

## 1. 配置

- [ ] 1.1 添加 Home Assistant 配置结构 (`internal/config/config.go`)
- [ ] 1.2 更新配置文件示例 (`configs/pibuddy.yaml`)

## 2. 工具实现

- [ ] 2.1 创建 Home Assistant 客户端 (`internal/tools/homeassistant.go`)
  - REST API 封装
  - 服务调用
  - 状态查询
- [ ] 2.2 实现控制设备工具
  - `ha_control_device`: 控制设备（开/关/调节）
  - `ha_get_device_state`: 查询设备状态
  - `ha_list_devices`: 列出设备

## 3. Pipeline 集成

- [ ] 3.1 在 `initTools` 中注册 Home Assistant 工具
- [ ] 3.2 更新系统提示词，添加智能家居控制能力说明

## 4. 测试

- [ ] 4.1 编写单元测试
- [ ] 4.2 集成测试（需要运行的 Home Assistant）

## 5. 文档

- [ ] 5.1 更新 README，说明 Home Assistant 部署和配置
- [ ] 5.2 添加设备类型支持列表

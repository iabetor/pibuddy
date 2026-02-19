# Change: 添加小米智能家居控制能力

## Why

PiBuddy 运行在树莓派上，用户希望通过语音控制米家智能设备（灯光、开关、空调、窗帘等）。小米于 2024 年 12 月官方开源了 Home Assistant 集成组件，为接入提供了便利条件。

## What Changes

- 添加 Home Assistant 工具，通过 REST API 控制米家设备
- 支持设备控制：开关、亮度调节、温度设置、状态查询
- 支持设备发现和状态同步
- 添加相关配置项

## Impact

- Affected specs: 新增 `xiaomi-smart-home` 能力
- Affected code:
  - `internal/tools/homeassistant.go` (新增)
  - `internal/config/config.go` (配置)
  - `internal/pipeline/pipeline.go` (工具注册)
  - `configs/pibuddy.yaml` (配置)

## Prerequisites

1. **部署 Home Assistant**
   - 在树莓派上安装 Home Assistant
   - 安装官方 xiaomi_home 集成
   - 使用小米账号登录授权

2. **配置说明**
   - 获取 Home Assistant 的 Long-Lived Access Token
   - 配置 PiBuddy 连接 Home Assistant API

## References

- [小米官方 Home Assistant 集成](https://github.com/XiaoMi/ha_xiaomi_home)
- [Home Assistant REST API](https://www.home-assistant.io/developers/rest_api/)
- [调研报告](./xiaomi-smart-home-research.md)

# Change: 多 LLM Provider 支持

## Why

当前 PiBuddy 只支持单一 LLM 配置，无法充分利用国内多个 LLM Provider 的免费额度。通过支持多 Provider 自动降级，可以最大化利用通义千问（400万 token）、腾讯混元（100万 token）、火山方舟（50万 token）等免费额度，降低运营成本。

## What Changes

- **新增多 Provider 支持**：配置中支持 `models` 列表，按优先级顺序使用
- **自动降级机制**：当前模型额度耗尽或请求失败时，自动切换到下一个
- **统一 OpenAI 兼容协议**：所有 Provider 使用 OpenAI 兼容 API，简化集成
- **额度耗尽检测**：检测 HTTP 402/429 和响应体关键词，触发切换
- **向后兼容**：保留旧配置格式，`models` 为空时使用单一配置

## Impact

- Affected specs: `llm`
- Affected code:
  - `internal/config/config.go` - 新增 LLMModelConfig 结构体
  - `internal/llm/multi_provider.go` - 新增 MultiProvider 实现
  - `internal/llm/openai.go` - 扩展错误检测
  - `internal/pipeline/pipeline.go` - MultiProvider 集成
  - `configs/pibuddy.yaml` - 多模型配置

## Provider 接入信息

### 1. 通义千问（阿里 DashScope）
- **API URL**: `https://dashscope.aliyuncs.com/compatible-mode/v1`
- **可用模型**: qwen-turbo、qwen-flash、qwen-plus、qwen-max
- **免费额度**: 每个模型各 100 万 token（2026/05/22 过期）

### 2. 腾讯混元
- **API URL**: `https://api.hunyuan.cloud.tencent.com/v1`
- **可用模型**: hunyuan-lite、hunyuan-turbo
- **免费额度**: 100 万 tokens（共享消耗）

### 3. 火山方舟（字节豆包）
- **API URL**: `https://ark.cn-beijing.volces.com/api/v3`
- **模型**: 使用接入点 ID (endpoint ID) 作为 model
- **免费额度**: 50 万 token

### 4. DeepSeek
- **API URL**: `https://api.deepseek.com/v1`
- **模型**: deepseek-chat
- **定位**: 付费模型，作为最终兜底

## 免费额度总计

| Provider | 模型数 | 额度 | 说明 |
|----------|--------|------|------|
| 通义千问 | 4 个 | 400 万 token | 每个模型各 100 万 |
| 腾讯混元 | 2 个 | 100 万 token | 共享额度 |
| 火山方舟 | 1 个接入点 | 50 万 token | 豆包全系 |
| **合计** | **7 个模型** | **550 万 token** | — |

按 PiBuddy 每次对话约 200-500 token 计算，可支持约 **11000-27500 次对话**。

## 配置示例

```yaml
llm:
  models:
    - name: "qwen-turbo"
      api_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
      api_key: "${PIBUDDY_QWEN_API_KEY}"
      model: "qwen-turbo"
    - name: "hunyuan-lite"
      api_url: "https://api.hunyuan.cloud.tencent.com/v1"
      api_key: "${PIBUDDY_HUNYUAN_API_KEY}"
      model: "hunyuan-lite"
    # ... 更多模型
```

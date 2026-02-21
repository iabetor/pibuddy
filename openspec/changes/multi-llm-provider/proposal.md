# 多 LLM Provider 方案

## 背景

当前 PiBuddy 只支持单一 LLM 配置（DeepSeek），需要扩展为支持多个 LLM Provider，实现：
- **多 Provider 优先级切换**：额度耗尽或请求失败时自动降级到下一个
- **国内四大 Provider 接入**：通义千问、腾讯混元、火山方舟、DeepSeek
- **最大化利用免费额度**：千问 400 万、混元 100 万、豆包 50 万，共 550 万 token

## Provider 接入信息

### 1. 通义千问（阿里 DashScope）
- **API URL**: `https://dashscope.aliyuncs.com/compatible-mode/v1`
- **协议**: OpenAI 兼容
- **可用模型**（每个各 100 万 token 免费额度，2026/05/22 过期）:
  - `qwen-turbo` — 快速响应，适合简单对话（优先）
  - `qwen-flash` — 极速，适合简单问答
  - `qwen-plus` — 高性能，日常对话主力
  - `qwen-max` — 旗舰级，复杂任务
- **排除模型**:
  - `qwq-plus`（推理型，输出思维链，响应慢，不适合语音助手）
  - `qwen3-coder-plus`（代码专用）
  - `qwen-vl-max`（视觉多模态）
- **额度耗尽标识**: HTTP 429 + quota 相关错误

### 2. 腾讯混元
- **API URL**: `https://api.hunyuan.cloud.tencent.com/v1`
- **协议**: OpenAI 兼容
- **可用模型**:
  - `hunyuan-lite` — 免费，适合日常对话
  - `hunyuan-turbo` — 快速响应，适合语音场景
- **免费额度**: 新用户 100 万 tokens（共享消耗）
- **额度耗尽标识**: HTTP 429

### 3. 火山方舟（字节豆包）
- **API URL**: `https://ark.cn-beijing.volces.com/api/v3`
- **协议**: OpenAI 兼容
- **模型**: 使用接入点 ID (endpoint ID) 作为 model，格式如 `ep-20260221xxxxxx`
- **免费额度**: 豆包全系模型 50 万 token
- **当前接入点**: `ep-20260221114820-w6fmj`

### 4. DeepSeek
- **API URL**: `https://api.deepseek.com/v1`
- **协议**: OpenAI 兼容
- **模型**: `deepseek-chat`
- **余额查询**: `GET https://api.deepseek.com/user/balance`
- **额度耗尽标识**: HTTP 402 + `Insufficient Balance`
- **定位**: 付费模型，作为最终兜底

## 设计方案

### 配置结构

```yaml
llm:
  # 多模型优先级列表，按顺序尝试，额度用完/请求失败自动切换到下一个
  models:
    # --- 通义千问（快速模型优先）---
    - name: "qwen-turbo"
      api_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
      api_key: "${PIBUDDY_QWEN_API_KEY}"
      model: "qwen-turbo"
    - name: "qwen-flash"
      api_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
      api_key: "${PIBUDDY_QWEN_API_KEY}"
      model: "qwen-flash"
    - name: "qwen-plus"
      api_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
      api_key: "${PIBUDDY_QWEN_API_KEY}"
      model: "qwen-plus"
    - name: "qwen-max"
      api_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
      api_key: "${PIBUDDY_QWEN_API_KEY}"
      model: "qwen-max"
    # --- 腾讯混元 ---
    - name: "hunyuan-lite"
      api_url: "https://api.hunyuan.cloud.tencent.com/v1"
      api_key: "${PIBUDDY_HUNYUAN_API_KEY}"
      model: "hunyuan-lite"
    - name: "hunyuan-turbo"
      api_url: "https://api.hunyuan.cloud.tencent.com/v1"
      api_key: "${PIBUDDY_HUNYUAN_API_KEY}"
      model: "hunyuan-turbo"
    # --- 火山方舟（豆包）---
    - name: "doubao"
      api_url: "https://ark.cn-beijing.volces.com/api/v3"
      api_key: "${PIBUDDY_ARK_API_KEY}"
      model: "${PIBUDDY_ARK_ENDPOINT_ID}"
    # --- DeepSeek（付费兜底）---
    - name: "deepseek-chat"
      api_url: "https://api.deepseek.com/v1"
      api_key: "${PIBUDDY_LLM_API_KEY}"
      model: "deepseek-chat"

  # 以下为兼容旧配置（当 models 列表为空时使用）
  provider: "openai"
  api_url: "https://api.deepseek.com/v1"
  api_key: "${PIBUDDY_LLM_API_KEY}"
  model: "deepseek-chat"

  system_prompt: |
    ...
  max_history: 10
  max_tokens: 500
```

### 环境变量配置

```bash
# 通义千问（阿里云 DashScope）
export PIBUDDY_QWEN_API_KEY="sk-c4ef18f25e1f46efa59bdb587aa48bdc"

# 腾讯混元
export PIBUDDY_HUNYUAN_API_KEY="sk-8HNw546UgO29yckJwrjQrKg9f8sWgKCLreDGvZZp17EAedEn"

# 火山方舟（字节豆包）
export PIBUDDY_ARK_API_KEY="9cf96e33-8cf0-4996-8a6b-1cc70ff8c8df"
export PIBUDDY_ARK_ENDPOINT_ID="ep-20260221114820-w6fmj"

# DeepSeek（付费兜底）
export PIBUDDY_LLM_API_KEY="你的DeepSeek密钥"
```

可添加到 `~/.zshrc` 或 `~/.bashrc` 中持久化。

### 切换策略

**顺序降级**（与 ASR 一致的简单可靠方案）：
1. 按 `models` 列表顺序使用，从第一个开始
2. 当前模型请求失败（额度耗尽/网络错误/超时）时，自动切换到下一个
3. 记住当前活跃模型索引，后续请求继续使用
4. 所有模型都失败时，播报错误提示

### 额度耗尽检测

统一检测以下情况并触发切换：
- HTTP 402（DeepSeek 余额不足）
- HTTP 429（速率限制/额度耗尽）
- 响应体包含 `insufficient`、`quota`、`balance` 等关键词
- 网络超时（>60s 无响应）

### 模型选择策略说明

**快速模型优先**：PiBuddy 是语音助手，延迟敏感，主要任务是意图识别+工具调用，不需要强推理能力：
- `qwen-turbo` / `qwen-flash`：响应快，适合 90% 的日常对话
- `qwen-plus` / `qwen-max`：更强大，处理复杂意图
- `hunyuan-lite` / `hunyuan-turbo`：腾讯混元，作为千问之后的备选
- `doubao`：豆包通用模型
- `deepseek-chat`：付费模型，最终兜底

**排除推理模型**：如 `qwq-plus`（输出思维链）、`DeepSeek-R1` 等，响应慢且不适合语音交互场景。

## 核心实现

### 1. `LLMModelConfig` 结构体

```go
type LLMModelConfig struct {
    Name   string `yaml:"name"`
    APIURL string `yaml:"api_url"`
    APIKey string `yaml:"api_key"`
    Model  string `yaml:"model"`
}
```

### 2. `MultiProvider` 包装层

```go
type MultiProvider struct {
    providers []providerEntry  // 按优先级排列
    current   int              // 当前活跃索引
    mu        sync.RWMutex
}

// ChatStreamWithTools 自动降级
func (m *MultiProvider) ChatStreamWithTools(...) {
    // 从当前索引开始尝试
    // 失败时切换到下一个，记录日志
    // 返回成功的结果
}
```

### 3. Pipeline 集成

Pipeline 初始化时：
- 如果 `models` 列表长度 > 1，创建 `MultiProvider`
- 如果 `models` 列表长度 = 1，直接使用 `OpenAIProvider`
- 否则使用旧的单一配置（向后兼容）

## 免费额度总计

| Provider | 模型数 | 额度 | 说明 |
|----------|--------|------|------|
| 通义千问 | 4 个 | 400 万 token | 每个模型各 100 万 |
| 腾讯混元 | 2 个 | 100 万 token | 共享额度 |
| 火山方舟 | 1 个接入点 | 50 万 token | 豆包全系 |
| **合计** | **7 个模型** | **550 万 token** | — |

按 PiBuddy 每次对话约 200-500 token 计算，可支持约 **11000-27500 次对话**。

## 后续增强（本期不实现）

- [ ] 使用统计：记录每个模型的调用次数、token 消耗、延迟
- [ ] DeepSeek 余额查询集成
- [ ] 通过工具命令查看统计："查看模型使用情况"
- [ ] 智能路由：根据 query 复杂度自动选择合适模型
- [ ] 余额权重选择：定期查询余额，优先用余额最多的模型（目前仅 DeepSeek 支持余额查询）

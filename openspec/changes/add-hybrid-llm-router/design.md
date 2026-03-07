# Design: Hybrid LLM Router

## Context
- Raspberry Pi 4B 8GB can run small LLMs (Qwen2.5-3B) at acceptable speeds
- pibuddy use cases include both simple tasks (storytelling) and complex tasks (reasoning)
- Cloud API costs and latency are concerns for high-frequency simple queries

## Goals
- Route simple queries to local LLM for speed and cost savings
- Route complex queries to cloud LLM for accuracy
- Provide graceful fallback when either provider fails
- Keep configuration simple and intuitive

## Non-Goals
- Real-time model switching or hot-loading
- Multi-model ensemble inference
- Automatic quality assessment of responses

## Decisions

### Decision 1: Task-Based Routing with Keyword Detection
**What**: Classify query complexity using keyword matching + task type hints
**Why**:
- Fast and deterministic (no extra LLM call for classification)
- Easy to configure and extend
- Sufficient for pibuddy's use cases

**Alternatives considered**:
- Local LLM self-classification: Adds latency, 3B models unreliable for classification
- ML classifier: Overkill for this use case, adds complexity

### Decision 2: Fallback Strategy
**What**: If local response contains uncertainty markers, retry with cloud
**Why**:
- Catches cases where keyword classification is wrong
- Provides safety net for quality

**Uncertainty markers**: "不知道", "不太清楚", "无法回答", "我不确定"

### Decision 3: Configuration Structure
```yaml
llm:
  # Local LLM (Ollama)
  local:
    enabled: true
    endpoint: http://localhost:11434/v1
    model: qwen2.5:3b
    timeout: 30s

  # Cloud LLM (existing providers)
  cloud:
    models:
      - name: deepseek-chat
        api_key: ${DEEPSEEK_API_KEY}
        # ... existing config

  # Routing rules
  routing:
    # Tasks always go to local
    local_tasks: [story, chat, greeting]
    # Tasks always go to cloud
    cloud_tasks: [reasoning, homework, explanation]
    # Keywords that trigger cloud routing
    complex_keywords:
      - "为什么"
      - "解释"
      - "分析"
      - "原因"
      - "怎么理解"
      - "比较"
      - "区别"
      - "数学题"
      - "作业"
```

### Decision 4: Provider Selection Algorithm
```
1. If task type is in local_tasks → use local
2. If task type is in cloud_tasks → use cloud
3. If query contains complex_keywords → use cloud
4. Otherwise → use local (default)
5. If local response is poor → fallback to cloud
6. If local fails to generate valid tool_call JSON → fallback to cloud
```

### Decision 5: Tool Calling with Local LLM
**What**: Tool calling tasks (play_music, get_weather) go through local LLM
**Why**:
- LLM is needed for: intent recognition + parameter extraction
- These are simple NLU tasks, no complex reasoning required
- Local 3B model is sufficient for structured output

**Risk**: Small models may produce malformed tool_call JSON
**Mitigation**: Fallback to cloud LLM if tool parsing fails

**Flow for tool tasks**:
```
"播放周杰伦的晴天" → Local LLM → tool_call(play_music, keyword="周杰伦晴天") → Execute Tool → Local LLM → "正在播放周杰伦的晴天"
```

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| Keyword-based routing may misclassify | Fallback to cloud on poor response |
| Local LLM may be slow on first request (cold start) | Keep Ollama service running, use systemd |
| Memory pressure on Raspberry Pi | Monitor memory, recommend 3B model max |
| Local model quality varies by task | User can disable local LLM if unsatisfied |

## Migration Plan
1. Add new configuration with `local.enabled: false` by default
2. Users opt-in by setting `local.enabled: true`
3. No breaking changes to existing cloud-only setup

## Open Questions
- [ ] Should we add a "mixed" mode where both providers are queried and responses are compared?
- [ ] Should response quality be logged for future routing optimization?
- [ ] Should we support custom complexity classifier functions?

# Change: Add Hybrid LLM Router (Local + Cloud)

## Why
Running all LLM requests through cloud APIs is costly and introduces latency. For simple tasks like storytelling and casual chat, a local LLM (e.g., Qwen2.5-3B running on Ollama) is sufficient and provides faster response times. Complex reasoning tasks still require cloud LLMs for accuracy.

The Raspberry Pi 4B 8GB can run small quantized models (1.5B-3B parameters) at 3-5 tokens/second, making local inference viable for simple tasks.

## What Changes
- Add task-based routing logic to classify query complexity
- Route simple tasks (storytelling, chat, greeting) to local LLM
- Route complex tasks (reasoning, homework, explanation) to cloud LLM
- Add fallback mechanism: if local LLM response is unsatisfactory, retry with cloud
- Add configuration options for local LLM endpoint and model selection

## Impact
- Affected specs: `llm`
- Affected code:
  - `internal/llm/` - new router module
  - `configs/pibuddy.yaml` - new configuration section
  - `internal/tools/` - may need routing hints

## Benefits
- Reduced API costs for simple tasks
- Lower latency for local responses
- Privacy: simple queries stay local
- Graceful degradation when cloud is unavailable

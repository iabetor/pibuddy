# Implementation Tasks

## 1. Configuration
- [ ] 1.1 Add `llm.local` configuration section in `configs/pibuddy.yaml`
- [ ] 1.2 Add `llm.routing` configuration for task-to-provider mapping
- [ ] 1.3 Define complexity keywords list for classification

## 2. Router Implementation
- [ ] 2.1 Create `internal/llm/router.go` with HybridRouter struct
- [ ] 2.2 Implement `classifyComplexity(query string)` function
- [ ] 2.3 Implement `SelectProvider(query string, taskType string)` method
- [ ] 2.4 Add fallback logic when local response is poor

## 3. Integration
- [ ] 3.1 Update `internal/llm/provider.go` to use HybridRouter
- [ ] 3.2 Add task type hints in tool handlers (optional)
- [ ] 3.3 Update existing chat handler to use routing

## 4. Testing
- [ ] 4.1 Unit tests for complexity classification
- [ ] 4.2 Unit tests for provider selection
- [ ] 4.3 Integration test with mock local/cloud providers
- [ ] 4.4 Manual test on Raspberry Pi with Ollama

## 5. Documentation
- [ ] 5.1 Update README with local LLM setup instructions
- [ ] 5.2 Document configuration options
- [ ] 5.3 Add example for Ollama integration

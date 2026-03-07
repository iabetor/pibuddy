## ADDED Requirements

### Requirement: Hybrid LLM Routing
The system SHALL support hybrid routing between local and cloud LLM providers based on query complexity and task type.

#### Scenario: Simple task routed to local LLM
- **WHEN** a query is classified as simple (e.g., storytelling, casual chat)
- **THEN** the system routes the request to the local LLM endpoint

#### Scenario: Complex task routed to cloud LLM
- **WHEN** a query contains complex reasoning keywords or is marked as a complex task type
- **THEN** the system routes the request to the cloud LLM provider

#### Scenario: Local LLM disabled
- **WHEN** `llm.local.enabled` is false
- **THEN** all requests are routed to cloud providers (existing behavior)

### Requirement: Query Complexity Classification
The system SHALL classify query complexity using keyword detection and task type hints.

#### Scenario: Keyword-based classification
- **WHEN** a query contains keywords like "为什么", "解释", "分析"
- **THEN** the query is classified as complex

#### Scenario: Task type hint
- **WHEN** a tool handler provides a task type hint (e.g., "homework")
- **THEN** the system uses the hint to determine routing

#### Scenario: Default to simple
- **WHEN** no complexity indicators are found
- **THEN** the query is classified as simple (routes to local)

### Requirement: Local LLM Fallback
The system SHALL fall back to cloud LLM when local response quality is poor.

#### Scenario: Uncertainty response detected
- **WHEN** local LLM response contains uncertainty markers (e.g., "不知道", "不太清楚")
- **THEN** the system automatically retries with cloud LLM

#### Scenario: Local LLM timeout
- **WHEN** local LLM does not respond within configured timeout
- **THEN** the system falls back to cloud LLM

#### Scenario: Local LLM error
- **WHEN** local LLM returns an error or is unavailable
- **THEN** the system falls back to cloud LLM

### Requirement: Local LLM Configuration
The system SHALL support configuration for local LLM endpoint and routing rules.

#### Scenario: Ollama endpoint configuration
- **WHEN** `llm.local.endpoint` is set to `http://localhost:11434/v1`
- **THEN** the system uses Ollama's OpenAI-compatible API

#### Scenario: Routing rules configuration
- **WHEN** `llm.routing.local_tasks` and `llm.routing.cloud_tasks` are configured
- **THEN** the system routes tasks according to the configuration

#### Scenario: Custom complexity keywords
- **WHEN** `llm.routing.complex_keywords` is configured
- **THEN** the system uses the custom keywords for complexity detection

### Requirement: Tool Calling Support
The system SHALL support tool calling (function calling) through local LLM for simple tool tasks.

#### Scenario: Music playback tool call
- **WHEN** user says "播放周杰伦的晴天"
- **THEN** local LLM generates `play_music` tool call with `keyword="周杰伦晴天"`

#### Scenario: Weather query tool call
- **WHEN** user says "今天北京天气怎么样"
- **THEN** local LLM generates `get_weather` tool call with `city="北京"`

#### Scenario: Tool call format error fallback
- **WHEN** local LLM produces malformed tool_call JSON
- **THEN** the system falls back to cloud LLM for the request

#### Scenario: Tool execution result processing
- **WHEN** tool execution returns a result
- **THEN** the result is sent to the same provider (local or cloud) that made the tool call

## ADDED Requirements

### Requirement: Multi-Provider LLM Support
The system SHALL support multiple LLM providers with automatic fallback when quota is exhausted or requests fail.

#### Scenario: Primary provider available
- **WHEN** a chat request is made
- **THEN** the system uses the first provider in the priority list

#### Scenario: Quota exhausted fallback
- **WHEN** the current provider returns HTTP 402 or 429 with quota-related error
- **THEN** the system automatically switches to the next provider in the list

#### Scenario: All providers failed
- **WHEN** all providers have failed
- **THEN** the system returns an error message to the user

### Requirement: Sequential Fallback Strategy
The system SHALL use a sequential fallback strategy for model selection.

#### Scenario: Provider selection
- **WHEN** a request is made
- **THEN** providers are tried in the order defined in configuration

#### Scenario: Current provider remembered
- **WHEN** a provider switch occurs
- **THEN** subsequent requests continue using the new current provider

### Requirement: OpenAI-Compatible API Protocol
All LLM providers SHALL use the OpenAI-compatible API protocol for unified integration.

#### Scenario: Qwen API integration
- **WHEN** using Alibaba Qwen models
- **THEN** requests are sent to DashScope OpenAI-compatible endpoint

#### Scenario: Hunyuan API integration
- **WHEN** using Tencent Hunyuan models
- **THEN** requests are sent to Hunyuan OpenAI-compatible endpoint

#### Scenario: Volcengine Ark API integration
- **WHEN** using ByteDance Doubao models
- **THEN** requests are sent to Ark API OpenAI-compatible endpoint with endpoint ID as model

### Requirement: Quota Exhaustion Detection
The system SHALL detect quota exhaustion across all providers.

#### Scenario: HTTP 402 detection
- **WHEN** a provider returns HTTP 402
- **THEN** the system treats it as quota exhaustion and triggers fallback

#### Scenario: HTTP 429 with quota keyword
- **WHEN** a provider returns HTTP 429 with "quota" or "insufficient" in response body
- **THEN** the system treats it as quota exhaustion and triggers fallback

#### Scenario: Network timeout
- **WHEN** a provider does not respond within 60 seconds
- **THEN** the system triggers fallback to the next provider

### Requirement: LLM Configuration
The LLM configuration SHALL support both single provider and multi-provider modes.

#### Scenario: Multi-provider mode
- **WHEN** the `models` list contains multiple entries
- **THEN** the system uses MultiProvider with automatic fallback

#### Scenario: Single-provider mode
- **WHEN** the `models` list contains exactly one entry
- **THEN** the system uses OpenAIProvider directly without fallback logic

#### Scenario: Legacy configuration mode
- **WHEN** the `models` list is empty or not present
- **THEN** the system uses legacy single-provider configuration for backward compatibility

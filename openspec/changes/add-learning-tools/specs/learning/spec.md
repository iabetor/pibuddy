## ADDED Requirements

### Requirement: English Word Query

The system SHALL provide English word lookup functionality with pronunciation and meaning.

#### Scenario: Query word meaning
- **WHEN** user asks "what does hello mean" or "hello 是什么意思"
- **THEN** the system returns the word meaning, phonetic transcription, and plays pronunciation

#### Scenario: Word not found
- **WHEN** user queries a non-existent word
- **THEN** the system returns a friendly error message

### Requirement: Daily English Quote

The system SHALL provide a daily English learning quote.

#### Scenario: Get daily quote
- **WHEN** user asks "每日一句" or "daily quote"
- **THEN** the system returns an English sentence with Chinese translation

### Requirement: Vocabulary Notebook

The system SHALL provide a personal vocabulary notebook for saving and reviewing words.

#### Scenario: Add word to notebook
- **WHEN** user says "把 apple 加入生词本"
- **THEN** the system adds the word to the vocabulary notebook

#### Scenario: List vocabulary
- **WHEN** user asks "复习生词本" or "生词本有什么"
- **THEN** the system lists all saved words with meanings

#### Scenario: Remove word
- **WHEN** user says "从生词本删除 apple"
- **THEN** the system removes the word from the notebook

### Requirement: English Word Quiz

The system SHALL provide an English word quiz game with a built-in word bank.

#### Scenario: Start quiz
- **WHEN** user says "来个单词测验"
- **THEN** the system starts a quiz session and asks for a word meaning

#### Scenario: Answer correctly
- **WHEN** user answers correctly
- **THEN** the system confirms and asks the next question

#### Scenario: Answer incorrectly
- **WHEN** user answers incorrectly
- **THEN** the system shows the correct answer and asks the next question

### Requirement: Chinese Pinyin Conversion

The system SHALL convert Chinese characters to Pinyin using a local library without network calls.

#### Scenario: Query character pronunciation
- **WHEN** user asks "龘 字怎么读"
- **THEN** the system returns the pinyin pronunciation

#### Scenario: Query phrase pinyin
- **WHEN** user asks "银行 的拼音是什么"
- **THEN** the system returns the pinyin for each character in the phrase

#### Scenario: Handle polyphonic characters
- **WHEN** a character has multiple pronunciations
- **THEN** the system returns all possible pronunciations with context

### Requirement: Daily Poetry

The system SHALL provide a daily Chinese poetry recommendation.

#### Scenario: Get daily poem
- **WHEN** user asks "每日一诗"
- **THEN** the system returns a poem with title, author, content, and optionally translation

### Requirement: Poetry Search

The system SHALL provide poetry search functionality by keyword, author, or famous lines.

#### Scenario: Search by keyword
- **WHEN** user asks "关于月亮的诗"
- **THEN** the system returns poems containing the keyword "月亮"

#### Scenario: Search next line
- **WHEN** user asks "春眠不觉晓 的下一句"
- **THEN** the system returns the next line and the source poem

#### Scenario: Search by author
- **WHEN** user asks "背一首李白的诗"
- **THEN** the system returns a poem by Li Bai

### Requirement: Poetry Games (Fei Hua Ling)

The system SHALL provide a "Fei Hua Ling" game where players take turns reciting poems containing a specified keyword.

#### Scenario: Start game
- **WHEN** user says "来玩飞花令，关键字 花"
- **THEN** the system starts the game and recites the first line containing "花"

#### Scenario: User responds correctly
- **WHEN** user recites a line containing the keyword
- **THEN** the system validates and responds with another line

#### Scenario: User responds incorrectly
- **WHEN** user's input does not contain the keyword
- **THEN** the system prompts for a valid response

### Requirement: Poetry Chain Game

The system SHALL provide a poetry chain game where each line must start with the last character of the previous line.

#### Scenario: Start chain game
- **WHEN** user says "来玩诗词接龙"
- **THEN** the system starts and recites the first line

#### Scenario: Valid chain response
- **WHEN** user recites a line starting with the last character (or homophone) of the previous line
- **THEN** the system accepts and responds with a connecting line

#### Scenario: Invalid chain response
- **WHEN** user's input does not connect properly
- **THEN** the system points out the connection requirement

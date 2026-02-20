## ADDED Requirements

### Requirement: Music Pause on Interrupt

The system SHALL pause music playback instead of stopping it when the wake word is detected during playback, preserving the playback state for later resumption.

#### Scenario: Wake word interrupts music
- **WHEN** music is playing and user says the wake word
- **THEN** the system pauses the music and preserves the playlist, current index, and play mode

#### Scenario: Non-wake-word interruption
- **WHEN** user explicitly says "停止播放" or "暂停音乐"
- **THEN** the system stops the music without preserving state (no resume available)

### Requirement: Resume Music Playback

The system SHALL provide a tool to resume paused music from the interrupted point.

#### Scenario: Resume after interrupt
- **WHEN** user says "继续播放" or "恢复播放" after an interrupt
- **THEN** the system resumes the paused music from the saved state

#### Scenario: Resume with expired URL
- **WHEN** user tries to resume but the song URL has expired
- **THEN** the system refreshes the URL and continues playing

#### Scenario: Resume with no paused music
- **WHEN** user says "继续播放" but no music was paused
- **THEN** the system responds "没有暂停的音乐"

#### Scenario: Resume after playlist ended
- **WHEN** user says "继续播放" but the previous playlist had finished
- **THEN** the system responds "播放列表已结束"

### Requirement: Preserve Playlist State

The system SHALL preserve the complete playlist state when music is paused by interrupt.

#### Scenario: Preserve playlist and position
- **WHEN** music is interrupted during the 3rd song in a 5-song playlist
- **THEN** the system saves all 5 songs, the current index (2), and the play mode

#### Scenario: Preserve play mode
- **WHEN** music is interrupted while in loop mode
- **THEN** the system saves the loop mode and resumes with the same mode

### Requirement: Clear Pause State on Explicit Stop

The system SHALL clear the pause state when user explicitly requests to stop or pause music.

#### Scenario: Explicit stop clears state
- **WHEN** user says "停止播放" or "暂停音乐"
- **THEN** the system clears the pause state and "继续播放" will not work

#### Scenario: New music replaces pause state
- **WHEN** user plays new music after an interrupt
- **THEN** the old pause state is replaced by the new playback

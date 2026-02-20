## ADDED Requirements

### Requirement: Music Favorites Storage

The system SHALL provide persistent storage for user favorite songs, with separate playlists for each identified user.

#### Scenario: Add song to favorites
- **WHEN** user says "收藏这首歌" or "我喜欢这首歌" while a song is playing
- **THEN** the system saves the current song to the user's favorite list

#### Scenario: Add to default favorites for unidentified user
- **WHEN** voice recognition fails to identify the user
- **THEN** the song is saved to the default "guest" favorites list

#### Scenario: Favorites persist after restart
- **WHEN** the system restarts
- **THEN** all previously saved favorites remain available

### Requirement: Play Favorites

The system SHALL allow users to play their saved favorite songs with random or sequential playback mode.

#### Scenario: Play favorites randomly
- **WHEN** user says "播放我的收藏" or "播放我喜欢的歌"
- **THEN** the system plays all saved favorites in random order

#### Scenario: Play favorites sequentially
- **WHEN** user says "顺序播放我的收藏"
- **THEN** the system plays favorites in the order they were added

#### Scenario: Empty favorites
- **WHEN** user tries to play favorites but the list is empty
- **THEN** the system responds "你的收藏是空的，先收藏一些歌曲吧"

### Requirement: List Favorites

The system SHALL allow users to view their saved favorite songs.

#### Scenario: List favorites
- **WHEN** user says "我收藏了哪些歌" or "我的收藏有什么"
- **THEN** the system lists all saved favorite songs with names and artists

#### Scenario: Empty favorites list
- **WHEN** user asks for favorites but has none saved
- **THEN** the system responds "你还没有收藏任何歌曲"

### Requirement: Remove from Favorites

The system SHALL allow users to remove songs from their favorites.

#### Scenario: Remove current song from favorites
- **WHEN** user says "把这首歌从收藏删掉" while a song is playing
- **THEN** the system removes the current song from favorites

#### Scenario: Remove non-existent song
- **WHEN** user tries to remove a song not in favorites
- **THEN** the system responds "这首歌不在你的收藏中"

### Requirement: Multi-user Favorites

The system SHALL maintain separate favorites lists for different users identified by voice recognition.

#### Scenario: Different users have separate favorites
- **WHEN** user A adds a song to favorites
- **AND** user B asks to play favorites
- **THEN** user B's favorites are played, not user A's

#### Scenario: Unidentified user uses default list
- **WHEN** voice recognition cannot identify the speaker
- **THEN** the default "guest" favorites list is used

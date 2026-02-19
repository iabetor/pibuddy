// Package rss 提供 RSS/Atom 订阅源管理和内容获取功能。
package rss

import "time"

// Feed 订阅源信息。
type Feed struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	AddedAt     time.Time `json:"added_at"`
	LastFetched time.Time `json:"last_fetched,omitempty"`
}

// FeedItem 订阅源条目。
type FeedItem struct {
	Title     string    `json:"title"`
	Summary   string    `json:"summary"`
	Link      string    `json:"link"`
	Published time.Time `json:"published"`
	FeedName  string    `json:"feed_name"`
}

package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Claude JSONL entry structures
type ClaudeEntry struct {
	Type      string        `json:"type"`
	Timestamp string        `json:"timestamp"`
	Message   ClaudeMessage `json:"message"`
}

type ClaudeMessage struct {
	ID    string       `json:"id"`
	Usage *ClaudeUsage `json:"usage,omitempty"`
}

type ClaudeUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// Daily stats structure
type DailyStats struct {
	Date                  string `json:"date"`
	TotalInputTokens      int64  `json:"totalInputTokens"`
	TotalOutputTokens     int64  `json:"totalOutputTokens"`
	TotalCacheWriteTokens int64  `json:"totalCacheWriteTokens"`
	TotalCacheReadTokens  int64  `json:"totalCacheReadTokens"`
	TotalTokens           int64  `json:"totalTokens"`
	RequestCount          int    `json:"requestCount"`
}

// Upload payload
type UsageData struct {
	Daily []DailyStats `json:"daily"`
}

// MessageDataEntry stores the last usage data for a message ID
type MessageDataEntry struct {
	DateStr string
	Usage   *ClaudeUsage
}

func collectUsageData() (*UsageData, error) {
	claudeDir := getClaudeProjectsDir()

	// Check if directory exists
	if _, err := os.Stat(claudeDir); os.IsNotExist(err) {
		return &UsageData{Daily: []DailyStats{}}, nil
	}

	// Phase 1: Store last usage per message ID (streaming creates multiple entries, last one has final values)
	// This matches the Python script logic: "Always overwrite - last entry has the final usage values"
	messageData := make(map[string]*MessageDataEntry)

	// Cutoff time (90 days)
	cutoffTime := time.Now().UTC().AddDate(0, 0, -90)

	// Find all JSONL files
	err := filepath.Walk(claudeDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if info.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}

		processJSONLFile(path, messageData, cutoffTime)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Phase 2: Aggregate by date using the last usage values
	dailyStatsMap := make(map[string]*DailyStats)
	for _, data := range messageData {
		dateStr := data.DateStr
		usage := data.Usage

		if dailyStatsMap[dateStr] == nil {
			dailyStatsMap[dateStr] = &DailyStats{Date: dateStr}
		}

		dailyStatsMap[dateStr].TotalInputTokens += int64(usage.InputTokens)
		dailyStatsMap[dateStr].TotalOutputTokens += int64(usage.OutputTokens)
		dailyStatsMap[dateStr].TotalCacheWriteTokens += int64(usage.CacheCreationInputTokens)
		dailyStatsMap[dateStr].TotalCacheReadTokens += int64(usage.CacheReadInputTokens)
		dailyStatsMap[dateStr].RequestCount++
	}

	// Convert map to sorted slice
	var dailyList []DailyStats
	for _, stats := range dailyStatsMap {
		stats.TotalTokens = stats.TotalInputTokens + stats.TotalOutputTokens +
			stats.TotalCacheWriteTokens + stats.TotalCacheReadTokens
		dailyList = append(dailyList, *stats)
	}

	sort.Slice(dailyList, func(i, j int) bool {
		return dailyList[i].Date < dailyList[j].Date
	})

	return &UsageData{Daily: dailyList}, nil
}

func processJSONLFile(path string, messageData map[string]*MessageDataEntry, cutoffTime time.Time) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Increase buffer size for large lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()

		var entry ClaudeEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		// Check if it's an assistant message
		if entry.Type != "assistant" {
			continue
		}

		// Parse timestamp
		if entry.Timestamp == "" {
			continue
		}

		msgTime, err := time.Parse(time.RFC3339, entry.Timestamp)
		if err != nil {
			// Try alternative format
			msgTime, err = time.Parse("2006-01-02T15:04:05.000Z", entry.Timestamp)
			if err != nil {
				continue
			}
		}

		// Check cutoff
		if msgTime.Before(cutoffTime) {
			continue
		}

		// Get date string for grouping
		dateStr := msgTime.Format("2006-01-02")

		// Check usage data (skip if usage is nil/empty - matches Python's "if not usage")
		usage := entry.Message.Usage
		if usage == nil {
			continue
		}

		// Use message ID as key, or generate one from timestamp if missing
		// This matches Python: key = msg_id if msg_id else f"no_id_{timestamp_str}"
		msgID := entry.Message.ID
		key := msgID
		if key == "" {
			key = "no_id_" + entry.Timestamp
		}

		// Always overwrite - last entry has the final usage values
		// This matches Python: "Always overwrite - last entry has the final usage values"
		messageData[key] = &MessageDataEntry{
			DateStr: dateStr,
			Usage:   usage,
		}
	}
}

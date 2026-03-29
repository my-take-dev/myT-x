package main

import "myT-x/internal/inputhistory"

const (
	// inputHistoryDir is the subdirectory under the config directory where
	// JSONL history files are stored.
	inputHistoryDir = inputhistory.Dir

	// inputHistoryMaxFiles caps the number of retained history files.
	inputHistoryMaxFiles = inputhistory.MaxFiles

	// inputHistoryMaxInputLen caps the rune count of a single history entry.
	inputHistoryMaxInputLen = inputhistory.MaxInputLen

	// shutdownFlushSentinel bypasses timer generation checks for forced shutdown flush.
	shutdownFlushSentinel = inputhistory.ShutdownFlushSentinel
)

// InputHistoryEntry represents a single input history record.
type InputHistoryEntry = inputhistory.Entry

package main

import (
	"fmt"
	"strings"
	"time"
)

func fmtDuration(duration time.Duration) string {
	result := []string{}
	started := false
	if duration > 7*24*time.Hour || started {
		started = true
		weeks := int(duration.Hours() / 24 / 7)
		duration %= 7 * 24 * time.Hour
		result = append(result, fmt.Sprintf("%2dw", weeks))
	}
	if duration > 24*time.Hour || started {
		started = true
		days := int(duration.Hours() / 24)
		duration %= 24 * time.Hour
		result = append(result, fmt.Sprintf("%2dd", days))
	}
	if duration > time.Hour || started {
		started = true
		hours := int(duration.Hours())
		duration %= time.Hour
		result = append(result, fmt.Sprintf("%2dh", hours))
	}
	if duration > time.Minute || started {
		started = true
		minutes := int(duration.Minutes())
		duration %= time.Minute
		result = append(result, fmt.Sprintf("%2dm", minutes))
	}
	seconds := int(duration.Seconds())
	result = append(result, fmt.Sprintf("%2ds", seconds))
	return strings.TrimSpace(strings.Join(result, ""))
}

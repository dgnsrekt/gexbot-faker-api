package notify

import (
	"fmt"
	"strings"
	"time"

	"github.com/dgnsrekt/gexbot-downloader/internal/download"
)

// FormatSuccessMessage creates a success notification body.
func FormatSuccessMessage(result *download.BatchResult, duration time.Duration) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Total: %d files\n", result.Total))
	sb.WriteString(fmt.Sprintf("Success: %d\n", result.Success))
	sb.WriteString(fmt.Sprintf("Skipped: %d\n", result.Skipped))
	sb.WriteString(fmt.Sprintf("Not Found: %d\n", result.NotFound))
	sb.WriteString(fmt.Sprintf("Duration: %s", duration.Round(time.Second)))

	return sb.String()
}

// FormatFailureMessage creates a failure notification body.
func FormatFailureMessage(result *download.BatchResult, duration time.Duration, err error) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Total: %d files\n", result.Total))
	sb.WriteString(fmt.Sprintf("Success: %d\n", result.Success))
	sb.WriteString(fmt.Sprintf("Failed: %d\n", result.Failed))
	sb.WriteString(fmt.Sprintf("Skipped: %d\n", result.Skipped))
	sb.WriteString(fmt.Sprintf("Duration: %s", duration.Round(time.Second)))

	if err != nil {
		sb.WriteString(fmt.Sprintf("\n\nError: %v", err))
	}

	// Include first 3 error messages if available
	if len(result.Errors) > 0 {
		sb.WriteString("\n\nErrors:\n")
		limit := 3
		if len(result.Errors) < limit {
			limit = len(result.Errors)
		}
		for i := 0; i < limit; i++ {
			sb.WriteString(fmt.Sprintf("- %s\n", result.Errors[i]))
		}
		if len(result.Errors) > 3 {
			sb.WriteString(fmt.Sprintf("... and %d more errors", len(result.Errors)-3))
		}
	}

	return sb.String()
}

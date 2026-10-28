package api

import (
	"context"

	"github.com/mona-actions/gh-migration-monitor/internal/models"
)

// ProgressFunc is called with all migrations fetched so far after each page.
type ProgressFunc func(migrations []models.Migration)

// GitHubClient defines the interface for GitHub API operations
type GitHubClient interface {
	// ListMigrations returns all migrations for the specified organization.
	// onProgress is optional and receives the migrations fetched so far after each page.
	ListMigrations(ctx context.Context, org string, isLegacy bool, onProgress ProgressFunc) ([]models.Migration, error)
}

// APIError represents an API error
type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Err        error  `json:"-"`
}

func (e *APIError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *APIError) Unwrap() error {
	return e.Err
}

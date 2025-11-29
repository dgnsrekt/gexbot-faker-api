package api

import "errors"

var (
	ErrNotFound    = errors.New("data not found for this ticker/date")
	ErrRateLimited = errors.New("rate limited by API")
	ErrAuthFailed  = errors.New("authentication failed")
)

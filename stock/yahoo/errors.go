package yahoo

import "errors"

var (
	// ErrNotFound means Yahoo has no quote for the symbol (unknown or delisted).
	ErrNotFound = errors.New("ticker not found")
	// ErrNoData means the symbol resolved but there is no recent chart to show.
	ErrNoData = errors.New("no chart data")
	// ErrUnavailable means Yahoo could not be reached or returned a server error.
	ErrUnavailable = errors.New("yahoo finance unavailable")
)

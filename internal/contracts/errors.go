package contracts

import "errors"

var (
	ErrNotFound             = errors.New("not found")
	ErrBlockedByFeatureFlag = errors.New("blocked by feature flag")
	ErrInvalidState         = errors.New("invalid state")
)

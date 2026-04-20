package provider

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

func formatHTTPProbeRequestError(component string, err error) string {
	component = strings.TrimSpace(component)
	cause := strings.TrimSpace(errorString(err))
	if cause == "" {
		return component + " health request could not be created"
	}
	lowerCause := strings.ToLower(cause)
	if strings.Contains(lowerCause, "missing protocol scheme") ||
		strings.Contains(lowerCause, "first path segment in url cannot contain colon") ||
		strings.Contains(lowerCause, "invalid url escape") ||
		strings.Contains(lowerCause, "missing ']'") ||
		strings.Contains(lowerCause, "invalid port") {
		return fmt.Sprintf("%s base_url is invalid: %s", component, cause)
	}
	return fmt.Sprintf("%s health request could not be created: %s", component, cause)
}

func formatHTTPProbeTransportError(component string, err error) string {
	component = strings.TrimSpace(component)
	cause := strings.TrimSpace(errorString(err))
	if cause == "" {
		return component + " health probe failed"
	}
	return fmt.Sprintf("%s health probe failed: %s", component, cause)
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr != nil && urlErr.Err != nil {
		return urlErr.Err.Error()
	}
	return err.Error()
}

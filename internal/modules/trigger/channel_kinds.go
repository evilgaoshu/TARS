package trigger

import "strings"

// IsDirectDeliveryChannelKind reports whether the channel kind can be
// delivered directly by the trigger worker without an adapter layer.
func IsDirectDeliveryChannelKind(value string) bool {
	switch strings.TrimSpace(value) {
	case "telegram", "in_app_inbox":
		return true
	default:
		return false
	}
}

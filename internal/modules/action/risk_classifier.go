package action

import (
	"fmt"
	"strings"
)

func validateCommand(command string, allowedPrefixes []string, blockedFragments []string, service string, serviceCommandAllowlist map[string][]string) error {
	normalized := strings.TrimSpace(command)
	if normalized == "" {
		return fmt.Errorf("command is empty")
	}

	if err := validateBlockedFragments(normalized, blockedFragments); err != nil {
		return err
	}

	if matchesCommandPrefix(normalized, allowedPrefixes) {
		return nil
	}

	service = strings.TrimSpace(service)
	if service != "" && matchesCommandPrefix(normalized, serviceCommandAllowlist[service]) {
		return nil
	}
	if service != "" && len(serviceCommandAllowlist[service]) > 0 {
		return fmt.Errorf("command prefix is not in service allowlist for %s", service)
	}
	return fmt.Errorf("command prefix is not in allowlist")
}

func validateBlockedFragments(command string, blockedFragments []string) error {
	lower := strings.ToLower(strings.TrimSpace(command))
	for _, fragment := range blockedFragments {
		if fragment == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(strings.TrimSpace(fragment))) {
			return fmt.Errorf("command contains blocked fragment: %s", fragment)
		}
	}
	return nil
}

func matchesCommandPrefix(command string, prefixes []string) bool {
	for _, prefix := range prefixes {
		trimmed := strings.TrimSpace(prefix)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(command, trimmed) {
			return true
		}
	}
	return false
}

func withDefaultPrefixes(items []string) []string {
	if len(items) > 0 {
		return items
	}

	return []string{
		"hostname",
		"uptime",
		"curl -fsS https://api.ipify.org",
		"systemctl status",
		"systemctl is-active",
		"journalctl -u",
		"cat /proc/loadavg",
		"df -h",
		"free -m",
		"ps ",
		"ss ",
		"echo ",
		"printf ",
	}
}

func withDefaultBlockedFragments(items []string) []string {
	if len(items) > 0 {
		return items
	}

	return []string{
		"rm -rf",
		"mkfs",
		"shutdown",
		"reboot",
		"poweroff",
		"dd if=",
		":(){",
		"curl | sh",
		"wget | sh",
		"iptables",
		"userdel",
	}
}

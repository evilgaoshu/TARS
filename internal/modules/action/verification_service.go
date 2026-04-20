package action

import "strings"

func VerificationServiceCandidates(service string) []string {
	trimmed := strings.TrimSpace(service)
	if trimmed == "" {
		return nil
	}

	candidates := []string{trimmed}
	switch strings.ToLower(trimmed) {
	case "ssh":
		candidates = append(candidates, "sshd")
	case "sshd":
		candidates = append(candidates, "ssh")
	case "postgres":
		candidates = append(candidates, "postgresql")
	case "postgresql":
		candidates = append(candidates, "postgres")
	}

	seen := make(map[string]struct{}, len(candidates))
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		normalized := strings.ToLower(strings.TrimSpace(candidate))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, candidate)
	}
	return result
}

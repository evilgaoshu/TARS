package reasoning

import (
	"regexp"
	"sort"
	"strings"
)

var (
	ipv4Pattern             = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	ipv6Pattern             = regexp.MustCompile(`\b(?:[0-9a-fA-F]{1,4}:){2,7}[0-9a-fA-F]{1,4}\b`)
	domainPattern           = regexp.MustCompile(`\b[a-zA-Z0-9][a-zA-Z0-9-]*(?:\.[a-zA-Z0-9-]+)+\b`)
	fileNamePattern         = regexp.MustCompile(`^[^/\s]+\.[A-Za-z0-9._*-]+$`)
	pathPattern             = regexp.MustCompile("(^|[\\s\"'=,(])((?:~?/|\\.{1,2}/)[^\\s\"',;|&<>]+)")
	secretAssignmentPattern = regexp.MustCompile(`(?i)\b(password|passwd|token|secret|api[_-]?key)\b\s*[:=]\s*([^\s,;]+)`)
	querySecretPattern      = regexp.MustCompile(`(?i)([?&](?:access[_-]?token|refresh[_-]?token|token|secret|api[_-]?key)=)([^&\s]+)`)
	bearerPattern           = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._=-]+\b`)
	basicAuthURLPattern     = regexp.MustCompile(`(?i)\b(https?://[^/\s:@]+:)([^@/\s]+)(@)`)
	skTokenPattern          = regexp.MustCompile(`\bsk-[A-Za-z0-9-]+\b`)
)

type desensitizer struct {
	valueToPlaceholder map[string]string
	placeholderToValue map[string]string
	hostCounter        int
	ipCounter          int
	pathCounter        int
	config             *DesensitizationConfig
	detections         *SensitiveDetections
}

func desensitizeContext(input map[string]interface{}) (map[string]interface{}, map[string]string) {
	return desensitizeContextWithConfig(input, nil)
}

func desensitizeContextWithConfig(input map[string]interface{}, cfg *DesensitizationConfig) (map[string]interface{}, map[string]string) {
	return desensitizeContextWithConfigAndDetections(input, cfg, nil)
}

func desensitizeContextWithConfigAndDetections(input map[string]interface{}, cfg *DesensitizationConfig, detections *SensitiveDetections) (map[string]interface{}, map[string]string) {
	if len(input) == 0 {
		return map[string]interface{}{}, nil
	}
	if cfg == nil {
		defaultCfg := DefaultDesensitizationConfig()
		cfg = &defaultCfg
	}
	if !cfg.Enabled {
		output := make(map[string]interface{}, len(input))
		for key, value := range input {
			output[key] = value
		}
		return output, nil
	}

	engine := &desensitizer{
		valueToPlaceholder: make(map[string]string),
		placeholderToValue: make(map[string]string),
		config:             cfg,
		detections:         normalizeSensitiveDetections(detections),
	}
	sanitized, _ := engine.sanitizeValue("", input).(map[string]interface{})
	return sanitized, cloneStringMap(engine.placeholderToValue)
}

func rehydratePlaceholders(input string, mapping map[string]string) string {
	return rehydratePlaceholdersWithConfig(input, mapping, nil)
}

func rehydratePlaceholdersWithConfig(input string, mapping map[string]string, cfg *DesensitizationConfig) string {
	if input == "" || len(mapping) == 0 {
		return input
	}
	if cfg == nil {
		defaultCfg := DefaultDesensitizationConfig()
		cfg = &defaultCfg
	}

	keys := make([]string, 0, len(mapping))
	for key := range mapping {
		if !allowPlaceholderRehydration(key, cfg) {
			continue
		}
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})

	output := input
	for _, placeholder := range keys {
		output = strings.ReplaceAll(output, placeholder, mapping[placeholder])
	}
	return output
}

func (d *desensitizer) sanitizeValue(key string, value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for childKey, childValue := range typed {
			out[childKey] = d.sanitizeValue(childKey, childValue)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i, item := range typed {
			out[i] = d.sanitizeValue(key, item)
		}
		return out
	case []map[string]interface{}:
		out := make([]map[string]interface{}, len(typed))
		for i, item := range typed {
			sanitized, _ := d.sanitizeValue(key, item).(map[string]interface{})
			out[i] = sanitized
		}
		return out
	case []string:
		out := make([]string, len(typed))
		for i, item := range typed {
			out[i] = d.sanitizeString(key, item)
		}
		return out
	case string:
		return d.sanitizeString(key, typed)
	default:
		return value
	}
}

func (d *desensitizer) sanitizeString(key string, input string) string {
	if input == "" {
		return input
	}

	sanitized := d.applyDetectionHints(input)
	sanitized = redactSecrets(sanitized, d.config)
	if sanitized == "" {
		return sanitized
	}

	if shouldReplaceWholePathValue(key, sanitized, d.config) {
		return d.placeholderFor("PATH", sanitized)
	}
	if shouldReplaceWholeHostValue(key, sanitized, d.config) {
		if ipv4Pattern.MatchString(sanitized) || ipv6Pattern.MatchString(sanitized) {
			return d.placeholderFor("IP", sanitized)
		}
		return d.placeholderFor("HOST", sanitized)
	}

	if d.config == nil || d.config.Placeholders.ReplaceInlinePath {
		sanitized = d.replaceInlinePaths(sanitized)
	}
	if d.config == nil || d.config.Placeholders.ReplaceInlineIP {
		sanitized = ipv4Pattern.ReplaceAllStringFunc(sanitized, func(match string) string {
			return d.placeholderFor("IP", match)
		})
		sanitized = ipv6Pattern.ReplaceAllStringFunc(sanitized, func(match string) string {
			return d.placeholderFor("IP", match)
		})
	}
	if d.config == nil || d.config.Placeholders.ReplaceInlineHost {
		sanitized = domainPattern.ReplaceAllStringFunc(sanitized, func(match string) string {
			return d.placeholderFor("HOST", match)
		})
	}

	return sanitized
}

func (d *desensitizer) applyDetectionHints(input string) string {
	if d == nil || d.detections == nil || input == "" {
		return input
	}

	output := input
	for _, value := range d.detections.Secrets {
		output = strings.ReplaceAll(output, value, "[REDACTED]")
	}
	for _, value := range d.detections.Paths {
		output = strings.ReplaceAll(output, value, d.placeholderFor("PATH", value))
	}
	for _, value := range d.detections.IPs {
		output = strings.ReplaceAll(output, value, d.placeholderFor("IP", value))
	}
	for _, value := range d.detections.Hosts {
		output = strings.ReplaceAll(output, value, d.placeholderFor("HOST", value))
	}
	return output
}

func (d *desensitizer) placeholderFor(kind string, value string) string {
	if placeholder, ok := d.valueToPlaceholder[value]; ok {
		return placeholder
	}

	var placeholder string
	switch kind {
	case "IP":
		d.ipCounter++
		placeholder = "[IP_" + intString(d.ipCounter) + "]"
	case "PATH":
		d.pathCounter++
		placeholder = "[PATH_" + intString(d.pathCounter) + "]"
	default:
		d.hostCounter++
		placeholder = "[HOST_" + intString(d.hostCounter) + "]"
	}

	d.valueToPlaceholder[value] = placeholder
	d.placeholderToValue[placeholder] = value
	return placeholder
}

func (d *desensitizer) replaceInlinePaths(input string) string {
	return pathPattern.ReplaceAllStringFunc(input, func(match string) string {
		prefix := ""
		candidate := match
		if len(match) > 0 {
			switch match[0] {
			case '/', '~', '.':
			default:
				prefix = match[:1]
				candidate = match[1:]
			}
		}
		if candidate == "" {
			return match
		}
		return prefix + d.placeholderFor("PATH", candidate)
	})
}

func shouldReplaceWholePathValue(key string, value string, cfg *DesensitizationConfig) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.ContainsAny(trimmed, " \t\r\n") {
		return false
	}

	lowerKey := strings.ToLower(strings.TrimSpace(key))
	fragments := []string{"path", "file", "filename", "dir", "directory"}
	if cfg != nil && len(cfg.Placeholders.PathKeyFragments) > 0 {
		fragments = cfg.Placeholders.PathKeyFragments
	}
	for _, fragment := range fragments {
		if strings.Contains(lowerKey, fragment) {
			if strings.Contains(trimmed, "/") || strings.HasPrefix(trimmed, ".") || fileNamePattern.MatchString(trimmed) {
				return true
			}
		}
	}
	return false
}

func shouldReplaceWholeHostValue(key string, value string, cfg *DesensitizationConfig) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.ContainsAny(trimmed, " \t\r\n") {
		return false
	}

	lowerKey := strings.ToLower(strings.TrimSpace(key))
	fragments := []string{"host", "hostname", "instance", "node", "address"}
	if cfg != nil && len(cfg.Placeholders.HostKeyFragments) > 0 {
		fragments = cfg.Placeholders.HostKeyFragments
	}
	for _, fragment := range fragments {
		if strings.Contains(lowerKey, fragment) {
			return true
		}
	}
	return false
}

func redactSecrets(input string, cfg *DesensitizationConfig) string {
	active := DefaultDesensitizationConfig()
	if cfg != nil {
		active = normalizeDesensitizationConfig(*cfg)
	}
	output := input
	if len(active.Secrets.KeyNames) > 0 {
		keyPattern := regexp.MustCompile(`(?i)\b(` + strings.Join(escapeRegexStrings(active.Secrets.KeyNames), "|") + `)\b\s*[:=]\s*([^\s,;]+)`)
		output = keyPattern.ReplaceAllString(output, `$1=[REDACTED]`)
	}
	if len(active.Secrets.QueryKeyNames) > 0 {
		queryPattern := regexp.MustCompile(`(?i)([?&](?:` + strings.Join(escapeRegexStrings(active.Secrets.QueryKeyNames), "|") + `)=)([^&\s]+)`)
		output = queryPattern.ReplaceAllString(output, `$1[REDACTED]`)
	}
	if active.Secrets.RedactBearer {
		output = bearerPattern.ReplaceAllStringFunc(output, func(match string) string {
			parts := strings.Fields(match)
			if len(parts) == 0 {
				return "[REDACTED]"
			}
			return parts[0] + " [REDACTED]"
		})
	}
	if active.Secrets.RedactBasicAuthURL {
		output = basicAuthURLPattern.ReplaceAllString(output, `$1[REDACTED]$3`)
	}
	if active.Secrets.RedactSKTokens {
		output = skTokenPattern.ReplaceAllString(output, "[REDACTED]")
	}
	for _, pattern := range active.Secrets.AdditionalPatterns {
		output = regexp.MustCompile(pattern).ReplaceAllString(output, "[REDACTED]")
	}
	return output
}

func allowPlaceholderRehydration(placeholder string, cfg *DesensitizationConfig) bool {
	switch {
	case strings.HasPrefix(placeholder, "[HOST_"):
		return cfg.Rehydration.Host
	case strings.HasPrefix(placeholder, "[IP_"):
		return cfg.Rehydration.IP
	case strings.HasPrefix(placeholder, "[PATH_"):
		return cfg.Rehydration.Path
	default:
		return true
	}
}

func escapeRegexStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out = append(out, regexp.QuoteMeta(trimmed))
	}
	return out
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func normalizeSensitiveDetections(input *SensitiveDetections) *SensitiveDetections {
	if input == nil {
		return nil
	}
	output := &SensitiveDetections{
		Secrets: sortByLengthDesc(uniqueNonEmptyStrings(input.Secrets)),
		Hosts:   sortByLengthDesc(uniqueNonEmptyStrings(input.Hosts)),
		IPs:     sortByLengthDesc(uniqueNonEmptyStrings(input.IPs)),
		Paths:   sortByLengthDesc(uniqueNonEmptyStrings(input.Paths)),
	}
	if len(output.Secrets) == 0 && len(output.Hosts) == 0 && len(output.IPs) == 0 && len(output.Paths) == 0 {
		return nil
	}
	return output
}

func uniqueNonEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	output := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		output = append(output, trimmed)
	}
	return output
}

func sortByLengthDesc(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	output := append([]string(nil), values...)
	sort.Slice(output, func(i, j int) bool {
		if len(output[i]) == len(output[j]) {
			return output[i] < output[j]
		}
		return len(output[i]) > len(output[j])
	})
	return output
}

func intString(value int) string {
	if value <= 0 {
		return "0"
	}

	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + (value % 10))
		value /= 10
	}
	return string(digits[index:])
}

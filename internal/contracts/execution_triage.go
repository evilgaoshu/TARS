package contracts

import (
	"sort"
	"strings"
	"time"
)

func SortExecutionsForTriage(items []ExecutionDetail, sortOrder string) {
	desc := strings.ToLower(strings.TrimSpace(sortOrder)) != "asc"
	sort.SliceStable(items, func(i, j int) bool {
		cmp := CompareExecutionTriage(items[i], items[j])
		if cmp == 0 {
			return false
		}
		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func CompareExecutionTriage(left ExecutionDetail, right ExecutionDetail) int {
	if delta := executionStateWeight(left.Status) - executionStateWeight(right.Status); delta != 0 {
		return delta
	}
	if delta := executionRiskWeight(left.RiskLevel) - executionRiskWeight(right.RiskLevel); delta != 0 {
		return delta
	}
	leftUpdated := executionTriageTime(left)
	rightUpdated := executionTriageTime(right)
	if !leftUpdated.Equal(rightUpdated) {
		if leftUpdated.After(rightUpdated) {
			return 1
		}
		return -1
	}
	return -strings.Compare(left.ExecutionID, right.ExecutionID)
}

func executionStateWeight(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pending":
		return 6
	case "executing":
		return 5
	case "approved":
		return 4
	case "failed", "timeout":
		return 3
	case "rejected":
		return 2
	case "completed":
		return 1
	default:
		return 0
	}
}

func executionRiskWeight(risk string) int {
	switch strings.ToLower(strings.TrimSpace(risk)) {
	case "critical":
		return 3
	case "warning":
		return 2
	default:
		return 1
	}
}

func executionTriageTime(detail ExecutionDetail) time.Time {
	for _, candidate := range []time.Time{detail.CompletedAt, detail.ApprovedAt, detail.CreatedAt} {
		if !candidate.IsZero() {
			return candidate
		}
	}
	return time.Time{}
}

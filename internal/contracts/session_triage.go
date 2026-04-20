package contracts

import (
	"sort"
	"strings"
	"time"
)

func SortSessionsForTriage(items []SessionDetail, sortOrder string) {
	desc := strings.ToLower(strings.TrimSpace(sortOrder)) != "asc"
	sort.SliceStable(items, func(i, j int) bool {
		cmp := CompareSessionTriage(items[i], items[j])
		if cmp == 0 {
			return false
		}
		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func CompareSessionTriage(left SessionDetail, right SessionDetail) int {
	if delta := sessionActiveWeight(left) - sessionActiveWeight(right); delta != 0 {
		return delta
	}
	if delta := sessionRiskWeight(left) - sessionRiskWeight(right); delta != 0 {
		return delta
	}
	if delta := sessionStateWeight(sessionQueueState(left)) - sessionStateWeight(sessionQueueState(right)); delta != 0 {
		return delta
	}
	leftUpdated := sessionUpdatedAt(left)
	rightUpdated := sessionUpdatedAt(right)
	if !leftUpdated.Equal(rightUpdated) {
		if leftUpdated.After(rightUpdated) {
			return 1
		}
		return -1
	}
	return -strings.Compare(sessionTriageHeadline(left), sessionTriageHeadline(right))
}

func sessionActiveWeight(detail SessionDetail) int {
	switch sessionQueueState(detail) {
	case "mitigated", "closed":
		return 0
	default:
		return 1
	}
}

func sessionQueueState(detail SessionDetail) string {
	executionPending := false
	for _, item := range detail.Executions {
		switch strings.ToLower(strings.TrimSpace(item.Status)) {
		case "pending", "approved", "executing":
			executionPending = true
		}
	}
	verificationStatus := ""
	if detail.Verification != nil {
		verificationStatus = strings.ToLower(strings.TrimSpace(detail.Verification.Status))
	}
	verificationSucceeded := verificationStatus == "completed" || verificationStatus == "resolved" || verificationStatus == "success"
	switch {
	case detail.Status == "failed" || verificationStatus == "failed" || verificationStatus == "error":
		return "manualIntervention"
	case detail.Status == "resolved":
		if verificationSucceeded {
			return "mitigated"
		}
		return "closed"
	case detail.Status == "pending_approval" || detail.Status == "executing" || executionPending:
		return "pendingExecution"
	case detail.Status == "verifying" || verificationSucceeded:
		return "confirmed"
	default:
		return "collectingEvidence"
	}
}

func sessionStateWeight(state string) int {
	switch state {
	case "pendingExecution":
		return 5
	case "manualIntervention":
		return 4
	case "collectingEvidence":
		return 3
	case "confirmed":
		return 2
	case "mitigated":
		return 1
	default:
		return 0
	}
}

func sessionRiskWeight(detail SessionDetail) int {
	risk := ""
	if detail.GoldenSummary != nil {
		risk = detail.GoldenSummary.Risk
	}
	if strings.TrimSpace(risk) == "" {
		risk = sessionRisk(detail)
	}
	switch strings.ToLower(strings.TrimSpace(risk)) {
	case "critical":
		return 4
	case "warning":
		return 3
	case "info":
		return 2
	case "low", "safe":
		return 1
	default:
		return 0
	}
}

func sessionUpdatedAt(detail SessionDetail) time.Time {
	latest := time.Time{}
	if detail.Verification != nil && detail.Verification.CheckedAt.After(latest) {
		latest = detail.Verification.CheckedAt
	}
	for _, item := range detail.Timeline {
		if item.CreatedAt.After(latest) {
			latest = item.CreatedAt
		}
	}
	for _, item := range detail.Executions {
		for _, candidate := range []time.Time{item.CompletedAt, item.ApprovedAt, item.CreatedAt} {
			if candidate.After(latest) {
				latest = candidate
			}
		}
	}
	return latest
}

func sessionTriageHeadline(detail SessionDetail) string {
	if detail.GoldenSummary != nil {
		if headline := strings.TrimSpace(detail.GoldenSummary.Headline); headline != "" {
			return headline
		}
		if conclusion := strings.TrimSpace(detail.GoldenSummary.Conclusion); conclusion != "" {
			return conclusion
		}
	}
	return strings.TrimSpace(detail.SessionID)
}

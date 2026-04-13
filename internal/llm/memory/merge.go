package memory

import (
	"coolaid/internal/store"
	"strings"
	"time"
)

func merge(current store.Memory, ex extraction) store.Memory {
	return store.Memory{
		ProjectSummary: mergeSummary(current.ProjectSummary, ex.SummaryUpdate),
		Topics:         mergeStringSet(current.Topics, ex.TopicsAdd, 20),
		Preferences:    mergeStringSet(current.Preferences, ex.PreferencesAdd, 10),
		UpdatedAt:      time.Now(),
	}
}

func mergeSummary(current, update string) string {
	update = strings.TrimSpace(update)
	if update == "" {
		return current
	}
	return update
}

func mergeStringSet(existing, add []string, limit int) []string {
	if len(add) == 0 {
		return existing
	}

	seen := make(map[string]struct{}, len(existing))
	out := make([]string, 0, len(existing)+len(add))

	// keep existing first (stable ordering)
	for _, v := range existing {
		v = normalize(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}

	// add new items
	for _, v := range add {
		v = normalize(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)

		if len(out) >= limit {
			break
		}
	}

	return out
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

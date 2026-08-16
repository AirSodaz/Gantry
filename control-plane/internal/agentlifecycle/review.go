package agentlifecycle

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type DiffEntry struct {
	Path     string `json:"path"`
	Change   string `json:"change"`
	Category string `json:"category"`
	Risk     string `json:"risk"`
	Before   any    `json:"before,omitempty"`
	After    any    `json:"after,omitempty"`
}

type DiffSummary struct {
	Total  int `json:"total"`
	High   int `json:"high"`
	Medium int `json:"medium"`
	Low    int `json:"low"`
}

type ReviewDecisionRequest struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

func buildDiff(before, after json.RawMessage) ([]DiffEntry, DiffSummary, error) {
	var left, right any
	if len(before) != 0 {
		if err := json.Unmarshal(before, &left); err != nil {
			return nil, DiffSummary{}, err
		}
	}
	if len(after) != 0 {
		if err := json.Unmarshal(after, &right); err != nil {
			return nil, DiffSummary{}, err
		}
	}
	entries := make([]DiffEntry, 0)
	diffValue("", left, right, &entries)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	summary := DiffSummary{Total: len(entries)}
	for _, entry := range entries {
		switch entry.Risk {
		case "high":
			summary.High++
		case "medium":
			summary.Medium++
		default:
			summary.Low++
		}
	}
	return entries, summary, nil
}

func diffValue(path string, before, after any, entries *[]DiffEntry) {
	if before == nil && after == nil {
		return
	}
	if before == nil {
		*entries = append(*entries, newDiff(path, "added", before, after))
		return
	}
	if after == nil {
		*entries = append(*entries, newDiff(path, "removed", before, after))
		return
	}
	left, lok := before.(map[string]any)
	right, rok := after.(map[string]any)
	if lok && rok {
		keys := make(map[string]struct{}, len(left)+len(right))
		for key := range left {
			keys[key] = struct{}{}
		}
		for key := range right {
			keys[key] = struct{}{}
		}
		ordered := make([]string, 0, len(keys))
		for key := range keys {
			ordered = append(ordered, key)
		}
		sort.Strings(ordered)
		for _, key := range ordered {
			child := path + "/" + escapeJSONPointer(key)
			diffValue(child, left[key], right[key], entries)
		}
		return
	}
	if leftArray, ok := before.([]any); ok {
		if rightArray, ok := after.([]any); ok {
			max := len(leftArray)
			if len(rightArray) > max {
				max = len(rightArray)
			}
			for i := 0; i < max; i++ {
				child := fmt.Sprintf("%s/%d", path, i)
				var l, r any
				if i < len(leftArray) {
					l = leftArray[i]
				}
				if i < len(rightArray) {
					r = rightArray[i]
				}
				diffValue(child, l, r, entries)
			}
			return
		}
	}
	if fmt.Sprint(before) != fmt.Sprint(after) {
		*entries = append(*entries, newDiff(path, "changed", before, after))
	}
}

func newDiff(path, change string, before, after any) DiffEntry {
	if path == "" {
		path = "/"
	}
	risk := "low"
	category := "metadata"
	security := []string{"/tools", "/network", "/credentials", "/commands", "/approval", "/runtime", "/permissions"}
	for _, prefix := range security {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			risk, category = "high", "security"
			break
		}
	}
	if category == "metadata" {
		category = "behavior"
		if path == "/mode" || strings.HasPrefix(path, "/mode/") {
			risk = "medium"
		}
	}
	return DiffEntry{Path: path, Change: change, Category: category, Risk: risk, Before: before, After: after}
}

func escapeJSONPointer(value string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(value)
}

func canonicalDigest(spec json.RawMessage) (json.RawMessage, string, error) {
	canonical, findings := ValidateSpec(spec)
	if len(findings) != 0 {
		return nil, "", ErrInvalidState
	}
	var normalized bytes.Buffer
	if err := json.Compact(&normalized, canonical); err != nil {
		return nil, "", err
	}
	return normalized.Bytes(), digest(normalized.Bytes()), nil
}

func digest(value []byte) string {
	var compact bytes.Buffer
	if err := json.Compact(&compact, value); err == nil {
		value = compact.Bytes()
	}
	result := sha256.Sum256(value)
	return "sha256:" + fmt.Sprintf("%x", result[:])
}

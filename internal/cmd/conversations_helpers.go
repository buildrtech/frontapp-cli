package cmd

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

func buildConvSearchQuery(c *ConvSearchCmd) (string, error) {
	if strings.TrimSpace(c.RawQuery) != "" {
		return strings.TrimSpace(c.RawQuery), nil
	}

	parts := make([]string, 0, 8)

	add := func(prefix, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}

		parts = append(parts, fmt.Sprintf("%s:%s", prefix, value))
	}

	add("from", c.From)
	add("to", c.To)
	add("recipient", c.Recipient)
	add("inbox", c.Inbox)
	for _, tag := range c.Tag {
		add("tag", tag)
	}

	if c.Status != "" {
		status := strings.ToLower(strings.TrimSpace(c.Status))
		switch status {
		case "open", "archived", "snoozed", "trashed":
			parts = append(parts, "is:"+status)
		default:
			return "", fmt.Errorf("invalid status: %s", c.Status)
		}
	}

	if c.Assignee != "" {
		parts = append(parts, "assignee:"+strings.TrimSpace(c.Assignee))
	}

	if c.Unassigned {
		parts = append(parts, "is:unassigned")
	}

	before, err := normalizeSearchTimeInput("before", c.Before)
	if err != nil {
		return "", err
	}

	after, err := normalizeSearchTimeInput("after", c.After)
	if err != nil {
		return "", err
	}

	add("before", before)
	add("after", after)

	if strings.TrimSpace(c.Query) != "" {
		parts = append(parts, strings.TrimSpace(c.Query))
	}

	if len(parts) == 0 {
		return "", fmt.Errorf("no query provided")
	}

	return strings.Join(parts, " "), nil
}

func normalizeSearchTimeInput(field, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}

	if isAllDigits(value) {
		return value, nil
	}

	if t, err := time.ParseInLocation("2006-01-02", value, time.Local); err == nil {
		return strconv.FormatInt(t.Unix(), 10), nil
	}

	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return strconv.FormatInt(t.Unix(), 10), nil
	}

	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return strconv.FormatInt(t.Unix(), 10), nil
	}

	return "", fmt.Errorf("invalid %s value %q: use Unix seconds, YYYY-MM-DD, or RFC3339", field, value)
}

func isAllDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}

	return value != ""
}

func readIDsFromInput(source string) ([]string, error) {
	if strings.TrimSpace(source) == "" {
		return nil, nil
	}

	if source != "-" {
		return nil, fmt.Errorf("unsupported ids-from: %s", source)
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("read ids from stdin: %w", err)
	}

	fields := strings.Fields(string(data))
	return fields, nil
}

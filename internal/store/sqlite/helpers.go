package sqlite

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/revittco/mcplexer/internal/store"
)

const timeFormat = time.RFC3339

func formatTime(t time.Time) string {
	return t.UTC().Format(timeFormat)
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(timeFormat, s)
	return t
}

func parseTimePtr(s *string) *time.Time {
	if s == nil {
		return nil
	}
	t := parseTime(*s)
	return &t
}

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := formatTime(*t)
	return &s
}

func normalizeJSON(data json.RawMessage, fallback string) string {
	if len(data) == 0 {
		return fallback
	}
	return string(data)
}

func checkRowsAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func mapConstraintError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "unique_") ||
		strings.Contains(msg, "already exists") {
		return store.ErrAlreadyExists
	}
	return err
}

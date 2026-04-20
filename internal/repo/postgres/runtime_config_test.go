package postgres

import (
	"database/sql"
	"errors"
	"testing"
	"time"
)

type setupStateRowStub struct {
	values []any
	err    error
}

func (s setupStateRowStub) Scan(dest ...any) error {
	if s.err != nil {
		return s.err
	}
	if len(dest) != len(s.values) {
		return errors.New("unexpected scan arity")
	}
	for i, value := range s.values {
		switch target := dest[i].(type) {
		case *bool:
			*target = value.(bool)
		case *string:
			*target = value.(string)
		case *[]byte:
			if value == nil {
				*target = nil
			} else {
				*target = append((*target)[:0], value.([]byte)...)
			}
		case *sql.NullTime:
			if value == nil {
				*target = sql.NullTime{}
			} else {
				*target = sql.NullTime{Time: value.(time.Time), Valid: true}
			}
		case *time.Time:
			*target = value.(time.Time)
		default:
			return errors.New("unsupported destination type")
		}
	}
	return nil
}

func TestScanSetupStateRowAllowsNilCompletedAt(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 8, 11, 0, 0, 0, time.UTC)
	state, err := scanSetupStateRow(setupStateRowStub{
		values: []any{
			false,
			"provider",
			"admin",
			"local_password",
			"primary-openai",
			"gpt-4o-mini",
			"inbox-primary",
			true,
			true,
			"ok",
			[]byte(`{"username":"admin","provider":"local_password","login_url":"/login"}`),
			nil,
			now,
		},
	})
	if err != nil {
		t.Fatalf("scan setup state: %v", err)
	}
	if !state.CompletedAt.IsZero() {
		t.Fatalf("expected zero completed_at for nil database value, got %v", state.CompletedAt)
	}
	if state.UpdatedAt != now {
		t.Fatalf("expected updated_at to round-trip, got %v", state.UpdatedAt)
	}
	if state.LoginHint.Provider != "local_password" {
		t.Fatalf("expected login hint to decode, got %+v", state.LoginHint)
	}
}


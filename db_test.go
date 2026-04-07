package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/lib/pq"
)

func TestIsPQError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		code string
		want bool
	}{
		{
			name: "matching unique violation",
			err:  &pq.Error{Code: pq.ErrorCode(pqUniqueViolation)},
			code: pqUniqueViolation,
			want: true,
		},
		{
			name: "matching FK violation",
			err:  &pq.Error{Code: pq.ErrorCode(pqForeignKeyViolation)},
			code: pqForeignKeyViolation,
			want: true,
		},
		{
			name: "non-matching code",
			err:  &pq.Error{Code: pq.ErrorCode("42P01")},
			code: pqUniqueViolation,
			want: false,
		},
		{
			name: "non-pq error",
			err:  errors.New("generic error"),
			code: pqUniqueViolation,
			want: false,
		},
		{
			name: "wrapped pq error",
			err:  fmt.Errorf("wrap: %w", &pq.Error{Code: pq.ErrorCode(pqUniqueViolation)}),
			code: pqUniqueViolation,
			want: true,
		},
		{
			name: "nil error",
			err:  nil,
			code: pqUniqueViolation,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isPQError(tt.err, tt.code)
			if got != tt.want {
				t.Errorf("isPQError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVacuumStepsDefined(t *testing.T) {
	t.Parallel()

	if len(vacuumSteps) == 0 {
		t.Fatal("vacuumSteps should not be empty")
	}

	for i, step := range vacuumSteps {
		if step.query == "" {
			t.Errorf("vacuumSteps[%d].query is empty", i)
		}
		if step.message == "" {
			t.Errorf("vacuumSteps[%d].message is empty", i)
		}
	}
}

func TestPQErrorCodes(t *testing.T) {
	t.Parallel()

	t.Run("unique violation code", func(t *testing.T) {
		t.Parallel()
		if pqUniqueViolation != "23505" {
			t.Errorf("pqUniqueViolation = %q, want %q", pqUniqueViolation, "23505")
		}
	})

	t.Run("foreign key violation code", func(t *testing.T) {
		t.Parallel()
		if pqForeignKeyViolation != "23503" {
			t.Errorf("pqForeignKeyViolation = %q, want %q", pqForeignKeyViolation, "23503")
		}
	})
}

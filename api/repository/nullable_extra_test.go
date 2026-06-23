package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Additional nullable.go coverage (branches not hit by nullable_test.go)
// ---------------------------------------------------------------------------

func TestNullTimeScan_ANSICLayout(t *testing.T) {
	var ptr *time.Time
	n := newNullTime(&ptr)

	// ANSIC layout in Go 1.25: "Mon Jan _2 15:04:05 2006" (no timezone)
	// March 15, 2025 is a Saturday
	err := n.Scan("Sat Mar 15 10:30:00 2025")
	assert.NoError(t, err)
	assert.NotNil(t, ptr)
}

func TestNullTimeScan_HighPrecisionLayout(t *testing.T) {
	var ptr *time.Time
	n := newNullTime(&ptr)

	// High-precision layout: "2006-01-02 15:04:05.999999999 -0700 MST"
	err := n.Scan("2025-03-15 10:30:00.123456789 +0000 UTC")
	assert.NoError(t, err)
	assert.NotNil(t, ptr)
}

func TestNullTimeScan_ByteSliceInvalid(t *testing.T) {
	var ptr *time.Time
	n := newNullTime(&ptr)

	// Invalid byte slice that sql.NullTime.Scan cannot parse
	err := n.Scan([]byte("not-a-time"))
	assert.Error(t, err)
}

func TestNullFloatScan_Float64Valid(t *testing.T) {
	var ptr *float64
	n := newNullFloat(&ptr)

	err := n.Scan(99.9)
	assert.NoError(t, err)
	assert.NotNil(t, ptr)
	assert.Equal(t, 99.9, *ptr)
}

func TestNullFloatScan_Int64Valid(t *testing.T) {
	var ptr *float64
	n := newNullFloat(&ptr)

	err := n.Scan(int64(42))
	assert.NoError(t, err)
	assert.NotNil(t, ptr)
	assert.Equal(t, float64(42), *ptr)
}

func TestNullFloatScan_ByteSliceValid(t *testing.T) {
	var ptr *float64
	n := newNullFloat(&ptr)

	err := n.Scan([]byte("3.14"))
	assert.NoError(t, err)
	assert.NotNil(t, ptr)
	assert.Equal(t, 3.14, *ptr)
}

func TestNullIntScan_Int64Valid(t *testing.T) {
	var ptr *int
	n := newNullInt(&ptr)

	err := n.Scan(int64(42))
	assert.NoError(t, err)
	assert.NotNil(t, ptr)
	assert.Equal(t, 42, *ptr)
}

func TestNullIntScan_ByteSliceValid(t *testing.T) {
	var ptr *int
	n := newNullInt(&ptr)

	err := n.Scan([]byte("123"))
	assert.NoError(t, err)
	assert.NotNil(t, ptr)
	assert.Equal(t, 123, *ptr)
}

func TestNullStringScan_StringValid(t *testing.T) {
	var ptr *string
	n := newNullString(&ptr)

	err := n.Scan("world")
	assert.NoError(t, err)
	assert.NotNil(t, ptr)
	assert.Equal(t, "world", *ptr)
}

func TestNullStringScan_ByteSliceValid(t *testing.T) {
	var ptr *string
	n := newNullString(&ptr)

	err := n.Scan([]byte("hello"))
	assert.NoError(t, err)
	assert.NotNil(t, ptr)
	assert.Equal(t, "hello", *ptr)
}

// ---------------------------------------------------------------------------
// sql_helpers.go coverage
// ---------------------------------------------------------------------------

func TestBuildPlaceholders(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want string
	}{
		{
			name: "zero returns empty",
			n:    0,
			want: "",
		},
		{
			name: "negative returns empty",
			n:    -1,
			want: "",
		},
		{
			name: "one placeholder",
			n:    1,
			want: "?",
		},
		{
			name: "two placeholders",
			n:    2,
			want: "?,?",
		},
		{
			name: "three placeholders",
			n:    3,
			want: "?,?,?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPlaceholders(tt.n)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToAnySlice(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []any
	}{
		{
			name:  "empty slice",
			input: []string{},
			want:  []any{},
		},
		{
			name:  "single element",
			input: []string{"active"},
			want:  []any{"active"},
		},
		{
			name:  "multiple elements",
			input: []string{"active", "pending", "conditioning"},
			want:  []any{"active", "pending", "conditioning"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toAnySlice(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

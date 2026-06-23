package repository

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNullTimeScan_Valid(t *testing.T) {
	var ptr *time.Time
	n := newNullTime(&ptr)

	ts := time.Date(2025, 3, 15, 10, 30, 0, 0, time.UTC)
	err := n.Scan(ts)
	assert.NoError(t, err)
	assert.NotNil(t, ptr)
	assert.True(t, ptr.Equal(ts))
}

func TestNullTimeScan_Null(t *testing.T) {
	var ptr *time.Time
	n := newNullTime(&ptr)

	err := n.Scan(any(nil))
	assert.NoError(t, err)
	assert.Nil(t, ptr)
}

func TestNullTimeScan_Error(t *testing.T) {
	var ptr *time.Time
	n := newNullTime(&ptr)

	err := n.Scan("not-a-time")
	assert.Error(t, err)
	assert.Nil(t, ptr)
}

func TestNullFloatScan_Valid(t *testing.T) {
	var ptr *float64
	n := newNullFloat(&ptr)

	val := 42.5
	err := n.Scan(val)
	assert.NoError(t, err)
	assert.NotNil(t, ptr)
	assert.Equal(t, val, *ptr)
}

func TestNullFloatScan_Null(t *testing.T) {
	var ptr *float64
	n := newNullFloat(&ptr)

	err := n.Scan(any(nil))
	assert.NoError(t, err)
	assert.Nil(t, ptr)
}

func TestNullFloatScan_Error(t *testing.T) {
	var ptr *float64
	n := newNullFloat(&ptr)

	err := n.Scan("not-a-float")
	assert.Error(t, err)
	assert.Nil(t, ptr)
}

func TestNullIntScan_Valid(t *testing.T) {
	var ptr *int
	n := newNullInt(&ptr)

	val := 100
	err := n.Scan(int64(val))
	assert.NoError(t, err)
	assert.NotNil(t, ptr)
	assert.Equal(t, val, *ptr)
}

func TestNullIntScan_Null(t *testing.T) {
	var ptr *int
	n := newNullInt(&ptr)

	err := n.Scan(any(nil))
	assert.NoError(t, err)
	assert.Nil(t, ptr)
}

func TestNullIntScan_Error(t *testing.T) {
	var ptr *int
	n := newNullInt(&ptr)

	err := n.Scan("not-an-int")
	assert.Error(t, err)
	assert.Nil(t, ptr)
}

func TestNullStringScan(t *testing.T) {
	tests := []struct {
		name     string
		src      any
		wantNil  bool
		wantVal  string
		wantErr  bool
	}{
		{
			name:    "valid string",
			src:     "hello",
			wantNil: false,
			wantVal: "hello",
		},
		{
			name:    "nil source",
			src:     nil,
			wantNil: true,
		},
		{
			name:    "empty string",
			src:     "",
			wantNil: false,
			wantVal: "",
		},
		{
			name:    "invalid type returns error",
			src:     struct{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ptr *string
			n := newNullString(&ptr)

			err := n.Scan(tt.src)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, ptr)
			} else {
				assert.NotNil(t, ptr)
				assert.Equal(t, tt.wantVal, *ptr)
			}
		})
	}
}

func TestNullTimeScan_StringInput(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantNil bool
		wantErr bool
	}{
		{
			name:    "RFC3339 string",
			src:     "2025-03-15T10:30:00Z",
			wantNil: false,
		},
		{
			name:    "DateTime string",
			src:     "2025-03-15 10:30:00",
			wantNil: false,
		},
		{
			name:    "empty string",
			src:     "",
			wantNil: true,
		},
		{
			name:    "unparseable string",
			src:     "not-a-time",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ptr *time.Time
			n := newNullTime(&ptr)

			err := n.Scan(tt.src)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, ptr)
			} else {
				assert.NotNil(t, ptr)
			}
		})
	}
}

func TestNullTimeScan_NullTimeInput(t *testing.T) {
	var ptr *time.Time
	n := newNullTime(&ptr)

	ts := time.Date(2025, 3, 15, 10, 30, 0, 0, time.UTC)
	err := n.Scan(ts)
	assert.NoError(t, err)
	assert.NotNil(t, ptr)
	assert.True(t, ptr.Equal(ts))
}

func TestNullTimeScan_NullTimeInvalid(t *testing.T) {
	var ptr *time.Time
	n := newNullTime(&ptr)

	err := n.Scan(nil)
	assert.NoError(t, err)
	assert.Nil(t, ptr)
}

func TestNullFloatScan_NullFloat64Invalid(t *testing.T) {
	var ptr *float64
	n := newNullFloat(&ptr)

	err := n.Scan(nil)
	assert.NoError(t, err)
	assert.Nil(t, ptr)
}

func TestNullIntScan_NullInt64Invalid(t *testing.T) {
	var ptr *int
	n := newNullInt(&ptr)

	err := n.Scan(nil)
	assert.NoError(t, err)
	assert.Nil(t, ptr)
}

func TestToNullString(t *testing.T) {
	tests := []struct {
		name  string
		input *string
		want  any
	}{
		{
			name:  "nil pointer returns nil",
			input: nil,
			want:  nil,
		},
		{
			name:  "valid pointer returns NullString",
			input: strPtr("hello"),
			want:  sql.NullString{String: "hello", Valid: true},
		},
		{
			name:  "empty string pointer",
			input: strPtr(""),
			want:  sql.NullString{String: "", Valid: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toNullString(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToNullFloat(t *testing.T) {
	tests := []struct {
		name  string
		input *float64
		want  any
	}{
		{
			name:  "nil pointer returns nil",
			input: nil,
			want:  nil,
		},
		{
			name:  "valid pointer returns NullFloat64",
			input: float64Ptr(42.5),
			want:  sql.NullFloat64{Float64: 42.5, Valid: true},
		},
		{
			name:  "zero float pointer",
			input: float64Ptr(0),
			want:  sql.NullFloat64{Float64: 0, Valid: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toNullFloat(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func strPtr(s string) *string {
	return &s
}

func float64Ptr(f float64) *float64 {
	return &f
}

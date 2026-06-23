package repository

import (
	"database/sql"
	"fmt"
	"time"
)

type nullTime struct {
	ptr    **time.Time
	nullAt sql.NullTime
}

func (n *nullTime) Scan(src any) error {
	if src == nil {
		*n.ptr = nil
		return nil
	}

	if t, ok := src.(time.Time); ok {
		*n.ptr = &t
		return nil
	}

	if s, ok := src.(string); ok {
		if s == "" {
			*n.ptr = nil
			return nil
		}
		for _, layout := range []string{time.RFC3339, time.DateTime, time.ANSIC, "2006-01-02 15:04:05.999999999 -0700 MST"} {
			if t, err := time.Parse(layout, s); err == nil {
				*n.ptr = &t
				return nil
			}
		}
		return fmt.Errorf("nullTime: cannot parse time string: %q", s)
	}

	if err := n.nullAt.Scan(src); err != nil {
		return err
	}
	if n.nullAt.Valid {
		val := n.nullAt.Time
		*n.ptr = &val
	} else {
		*n.ptr = nil
	}

	return nil
}

func newNullTime(t **time.Time) *nullTime {
	return &nullTime{ptr: t}
}

type nullFloat struct {
	ptr    **float64
	nullAt sql.NullFloat64
}

func (n *nullFloat) Scan(src any) error {
	if err := n.nullAt.Scan(src); err != nil {
		return err
	}
	if n.nullAt.Valid {
		val := n.nullAt.Float64
		*n.ptr = &val
	} else {
		*n.ptr = nil
	}

	return nil
}

func newNullFloat(f **float64) *nullFloat {
	return &nullFloat{ptr: f}
}

type nullInt struct {
	ptr    **int
	nullAt sql.NullInt64
}

func (n *nullInt) Scan(src any) error {
	if err := n.nullAt.Scan(src); err != nil {
		return err
	}
	if n.nullAt.Valid {
		val := int(n.nullAt.Int64)
		*n.ptr = &val
	} else {
		*n.ptr = nil
	}

	return nil
}

func newNullInt(i **int) *nullInt {
	return &nullInt{ptr: i}
}

type nullString struct {
	ptr    **string
	nullAt sql.NullString
}

func (n *nullString) Scan(src any) error {
	if err := n.nullAt.Scan(src); err != nil {
		return err
	}
	if n.nullAt.Valid {
		val := n.nullAt.String
		*n.ptr = &val
	} else {
		*n.ptr = nil
	}
	return nil
}

func newNullString(s **string) *nullString {
	return &nullString{ptr: s}
}

func toNullString(val *string) any {
	if val == nil {
		return nil
	}
	return sql.NullString{String: *val, Valid: true}
}

func toNullFloat(val *float64) any {
	if val == nil {
		return nil
	}
	return sql.NullFloat64{Float64: *val, Valid: true}
}

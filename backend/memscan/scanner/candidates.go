// Copyright (C) 2026 naparuba This program is free software

package scanner

import (
	"math"
	"strconv"
	"strings"
)

const (
	// integers up to these magnitudes are exactly representable as floats
	maxExactFloat32 = 1 << 24
	maxExactFloat64 = 1 << 53
)

func ParseCandidates(raw string, option Option, allowZero bool) []*Value {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}
	var values []*Value
	if isInteger(s) {
		values = integerCandidates(s, allowZero)
	} else if isDecimal(s) {
		values = decimalCandidates(s, allowZero)
	}
	for _, v := range values {
		v.WithOption(option)
	}
	return values
}

// ParseAs is ParseCandidates restricted to one type
func ParseAs(raw string, typ Type, option Option, allowZero bool) *Value {
	for _, v := range ParseCandidates(raw, option, allowZero) {
		if v.Type() == typ {
			return v
		}
	}
	return nil
}

// isInteger accepts an optional sign followed by decimal digits
func isInteger(s string) bool {
	if s[0] == '+' || s[0] == '-' {
		s = s[1:]
	}
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// isDecimal accepts what a person types for a decimal number
func isDecimal(s string) bool {
	digits := false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			digits = true
		case c == '+' || c == '-' || c == '.' || c == 'e' || c == 'E':
		default:
			return false
		}
	}
	return digits
}

func integerCandidates(s string, allowZero bool) (values []*Value) {
	add := func(v *Value, unsigned bool) {
		v.SetUnsigned(unsigned)
		values = append(values, v)
	}

	if s[0] == '-' {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil
		}
		if n == 0 {
			return zeroCandidates(allowZero)
		}
		if n >= math.MinInt8 {
			add(NewInt8(int8(n)), false)
		}
		if n >= math.MinInt16 {
			add(NewInt16(int16(n)), false)
		}
		if n >= math.MinInt32 {
			add(NewInt32(int32(n)), false)
		}
		add(NewInt64(n), false)
		if n >= -maxExactFloat32 {
			values = append(values, NewFloat32(float32(n)))
		}
		if n >= -maxExactFloat64 {
			values = append(values, NewFloat64(float64(n)))
		}
		return
	}

	u, err := strconv.ParseUint(strings.TrimPrefix(s, "+"), 10, 64)
	if err != nil {
		return nil
	}
	if u == 0 {
		return zeroCandidates(allowZero)
	}
	if u <= math.MaxUint8 {
		add(NewInt8(int8(u)), true)
	}
	if u <= math.MaxUint16 {
		add(NewInt16(int16(u)), true)
	}
	if u <= math.MaxUint32 {
		add(NewInt32(int32(u)), true)
	}
	add(NewInt64(int64(u)), true)
	if u <= maxExactFloat32 {
		values = append(values, NewFloat32(float32(u)))
	}
	if u <= maxExactFloat64 {
		values = append(values, NewFloat64(float64(u)))
	}
	return
}

func decimalCandidates(s string, allowZero bool) (values []*Value) {
	f64, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsInf(f64, 0) || math.IsNaN(f64) {
		return nil
	}
	if f64 == 0 {
		if !allowZero {
			return nil
		}
		return []*Value{NewFloat32(0), NewFloat64(0)}
	}
	// ParseFloat reports an overflow of float32 as an error
	if f32, err := strconv.ParseFloat(s, 32); err == nil && f32 != 0 {
		values = append(values, NewFloat32(float32(f32)))
	}
	return append(values, NewFloat64(f64))
}

// zeroCandidates: 0 fits every type
func zeroCandidates(allowZero bool) []*Value {
	if !allowZero {
		return nil
	}
	values := []*Value{NewInt8(0), NewInt16(0), NewInt32(0), NewInt64(0), NewFloat32(0), NewFloat64(0)}
	for _, v := range values[:4] {
		v.SetUnsigned(true)
	}
	return values
}

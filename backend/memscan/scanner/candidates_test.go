package scanner

import (
	"reflect"
	"testing"
)

func typesOf(values []*Value) (types []Type) {
	for _, v := range values {
		types = append(types, v.Type())
	}
	return
}

func TestParseCandidatesTypes(t *testing.T) {
	all := []Type{Int8, Int16, Int32, Int64, Float32, Float64}
	tests := []struct {
		input string
		want  []Type
	}{
		// positive integers: unsigned range of each type, exact floats
		{"100", all},
		{"255", all},
		{"256", []Type{Int16, Int32, Int64, Float32, Float64}},
		{"65535", []Type{Int16, Int32, Int64, Float32, Float64}},
		{"65536", []Type{Int32, Int64, Float32, Float64}},
		{"16777216", []Type{Int32, Int64, Float32, Float64}}, // 2^24: last exact float32
		{"16777217", []Type{Int32, Int64, Float64}},          // 2^24+1: not a float32
		{"4294967295", []Type{Int32, Int64, Float64}},        // max uint32
		{"4294967296", []Type{Int64, Float64}},               //
		{"9007199254740992", []Type{Int64, Float64}},         // 2^53: last exact float64
		{"9007199254740993", []Type{Int64}},                  //
		{"18446744073709551615", []Type{Int64}},              // max uint64
		{"18446744073709551616", nil},                        // overflows everything
		{"+5", all},                                          //
		{"007", all},                                         // leading zeros
		{"  42  ", all},                                      // surrounding blanks
		// negative integers: signed range
		{"-1", all},
		{"-128", all},
		{"-129", []Type{Int16, Int32, Int64, Float32, Float64}},
		{"-32769", []Type{Int32, Int64, Float32, Float64}},
		{"-2147483649", []Type{Int64, Float64}},
		{"-9223372036854775808", []Type{Int64}},
		{"-9223372036854775809", nil},
		// decimals
		{"3.5", []Type{Float32, Float64}},
		{"-3.5", []Type{Float32, Float64}},
		{".5", []Type{Float32, Float64}},
		{"5.", []Type{Float32, Float64}},
		{"1e3", []Type{Float32, Float64}},
		{"1E-3", []Type{Float32, Float64}},
		{"1e50", []Type{Float64}},  // overflows a float32
		{"1e-50", []Type{Float64}}, // would become 0 in a float32
		{"1e400", nil},             // overflows a float64
		// not numbers
		{"", nil},
		{"   ", nil},
		{"abc", nil},
		{"12abc", nil},
		{"1,5", nil},
		{"0x10", nil},
		{"0x1p-2", nil},
		{"1_000", nil},
		{"NaN", nil},
		{"inf", nil},
		{"-Inf", nil},
		{"--5", nil},
		{"+-5", nil},
		{"1-5", nil},
		{"1.2.3", nil},
		{"e", nil},
		{".", nil},
		{"-", nil},
		// zero is refused when allowZero is false (checked below)
	}
	for _, tc := range tests {
		got := typesOf(ParseCandidates(tc.input, OptionFloatUnrounded, false))
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q: types %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseCandidatesZero(t *testing.T) {
	all := []Type{Int8, Int16, Int32, Int64, Float32, Float64}
	for _, input := range []string{"0", "000", "-0", "+0"} {
		if got := ParseCandidates(input, OptionFloatUnrounded, false); got != nil {
			t.Errorf("%q: %v accepted although zero is refused", input, typesOf(got))
		}
		if got := typesOf(ParseCandidates(input, OptionFloatUnrounded, true)); !reflect.DeepEqual(got, all) {
			t.Errorf("%q with allowZero: %v, want every type", input, got)
		}
	}
	for _, input := range []string{"0.0", "0e5", "-0.0"} {
		if got := ParseCandidates(input, OptionFloatUnrounded, false); got != nil {
			t.Errorf("%q: accepted although zero is refused", input)
		}
		if got := typesOf(ParseCandidates(input, OptionFloatUnrounded, true)); !reflect.DeepEqual(got, []Type{Float32, Float64}) {
			t.Errorf("%q with allowZero: %v, want the float types", input, got)
		}
	}
}

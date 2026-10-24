// Copyright (C) 2026 naparuba This program is free software

package scanner

import "bytes"

// ChunkMatcher finds the occurrences of one Value inside an in-memory chunk
type ChunkMatcher interface {
	Type() Type
	ScanChunk(chunk []byte, offset int, collector CollectorFunc)
}

func NewChunkMatcher(value Value) ChunkMatcher {
	switch {
	case value.typ == Bytes:
		return nil
	case value.typ == Float32 && value.HasOption():
		rf := newRoundedFloat32(value)
		return &float32RangeMatcher{min: rf.min, max: rf.max}
	case value.typ == Float64 && value.HasOption():
		rf := newRoundedFloat64(value)
		return &float64RangeMatcher{min: rf.min, max: rf.max}
	default:
		return &exactMatcher{
			typ:     value.typ,
			pattern: append([]byte(nil), value.data...),
		}
	}
}

// exactMatcher looks for the raw bytes of a value, on the value's alignment
type exactMatcher struct {
	typ     Type
	pattern []byte
}

func (m *exactMatcher) Type() Type { return m.typ }

func (m *exactMatcher) ScanChunk(chunk []byte, offset int, collector CollectorFunc) {
	size := len(m.pattern)
	if size == 0 {
		return
	}
	for i := 0; i+size <= len(chunk); {
		n := bytes.Index(chunk[i:], m.pattern)
		if n < 0 {
			return
		}
		pos := i + n
		if misalign := (offset + pos) % size; misalign != 0 {
			// not on a value boundary: resume at the next boundary
			i = pos + (size - misalign)
			continue
		}
		if !collector(offset + pos) {
			return
		}
		i = pos + size
	}
}

// float32RangeMatcher matches float32 values in [min, max)
type float32RangeMatcher struct {
	min, max float32
}

func (m *float32RangeMatcher) Type() Type { return Float32 }

func (m *float32RangeMatcher) ScanChunk(chunk []byte, offset int, collector CollectorFunc) {
	if n := len(chunk) / 4 * 4; n >= 4 {
		scanFloat32Vectorized(chunk[:n], m.min, m.max, offset, collector)
	}
}

// float64RangeMatcher matches float64 values in [min, max)
type float64RangeMatcher struct {
	min, max float64
}

func (m *float64RangeMatcher) Type() Type { return Float64 }

func (m *float64RangeMatcher) ScanChunk(chunk []byte, offset int, collector CollectorFunc) {
	if n := len(chunk) / 8 * 8; n >= 8 {
		scanFloat64Vectorized(chunk[:n], m.min, m.max, offset, collector)
	}
}

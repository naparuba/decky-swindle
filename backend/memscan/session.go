// Copyright (C) 2026 naparuba This program is free software

package memscan

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/kayon/memscan/deck"
	"github.com/kayon/memscan/scanner"
)

const (
	// sessionChunkSize must stay a multiple of 8, see scanner.ChunkMatcher
	sessionChunkSize = 1 << 16

	// per-type result buffers start small and grow when results are merged
	sessionInitialResultsCaps = 1 << 16
)

var (
	ErrSessionNotOpen  = errors.New("memscan: session is not open")
	ErrProcessGone     = errors.New("memscan: process does not exist")
	ErrNoValues        = errors.New("memscan: no value to scan")
	ErrUnsupportedType = errors.New("memscan: type not supported by multi-type scan")
	ErrDuplicateType   = errors.New("memscan: the same type was given twice")
	ErrScanCanceled    = errors.New("memscan: scan canceled")
)

// DefaultResultLimits: ~32 MB, ~256 MB and ~512 MB of addresses at most
var DefaultResultLimits = ResultLimits{Soft: 4 << 20, Hard: 32 << 20, Global: 64 << 20}

var sessionChunkPool = sync.Pool{New: func() any {
	b := make([]byte, sessionChunkSize)
	return &b
}}

var sessionScanTypesInOrder = [...]scanner.Type{
	scanner.Int8, scanner.Int16, scanner.Int32, scanner.Int64, scanner.Float32, scanner.Float64,
}

// ResultLimits bounds the number of addresses kept after a first scan
type ResultLimits struct {
	// Soft: above it a type is flagged StatusParked
	Soft int
	// Hard: above it a type is dropped (StatusTruncated)
	Hard int
	// Global: sum over all types
	Global int
}

// TypeStatus tells how the results of one type must be considered
type TypeStatus uint8

const (
	StatusOK TypeStatus = iota
	// StatusParked: more results than ResultLimits.Soft, kept but too many to list
	StatusParked
	// StatusTruncated: more results than the limits allow, they were dropped
	StatusTruncated
	// StatusEmpty: the last scan left no result for this type
	StatusEmpty
	// StatusDropped: the value of the last scan cannot be expressed in this type
	StatusDropped
)

// Active tells whether the type still has results that can be refined
func (s TypeStatus) Active() bool {
	return s == StatusOK || s == StatusParked
}

func (s TypeStatus) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusParked:
		return "parked"
	case StatusTruncated:
		return "truncated"
	case StatusEmpty:
		return "empty"
	case StatusDropped:
		return "dropped"
	}
	return "unknown"
}

// TypeInfo summarizes the state of one type of a Session
type TypeInfo struct {
	Type   scanner.Type
	Count  int
	Status TypeStatus
}

// Session scans one process for several value types at once
type Session struct {
	proc *deck.Process
	maps *Maps

	scans  map[scanner.Type]*Memscan
	status map[scanner.Type]TypeStatus
	round  uint
	undo   *undoState

	limits ResultLimits
	sem    *semaphore.Weighted
	ctx    context.Context
	cancel context.CancelFunc

	// how the process is paused, see SetPauseFuncs
	pause       func() error
	resume      func() error
	customPause bool
}

func NewSession() *Session {
	s := &Session{
		limits: DefaultResultLimits,
		sem:    semaphore.NewWeighted(int64(runtime.NumCPU()) * 4),
	}
	s.Reset()
	return s
}

func (s *Session) Open(proc *deck.Process) error {
	_ = s.Close()

	maps, err := OpenMaps(proc.PID)
	if err != nil {
		return err
	}
	s.proc = proc
	s.maps = maps
	if !s.customPause {
		s.pause = proc.Pause
		s.resume = proc.Resume
	}
	return nil
}

// SetPauseFuncs replaces how the process is paused during a first scan
func (s *Session) SetPauseFuncs(pause, resume func() error) {
	s.pause, s.resume = pause, resume
	s.customPause = true
}

func (s *Session) Close() (err error) {
	s.Reset()
	if s.maps != nil {
		err = s.maps.Close()
		s.maps = nil
	}
	s.proc = nil
	return
}

func (s *Session) Cancel() {
	if s.cancel != nil {
		s.cancel()
	}
}

// Reset forgets every result
func (s *Session) Reset() {
	s.Cancel()
	s.ctx, s.cancel = context.WithCancel(context.Background())
	for _, m := range s.scans {
		m.release()
	}
	s.scans = make(map[scanner.Type]*Memscan)
	s.status = make(map[scanner.Type]TypeStatus)
	s.round = 0
	s.undo = nil
}

// renewContext gives the session
func (s *Session) renewContext() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	for _, m := range s.scans {
		m.ctx, m.cancel = s.ctx, s.cancel
	}
}

// Scan returns the single-type scan holding the results of typ
func (s *Session) Scan(typ scanner.Type) *Memscan {
	return s.scans[typ]
}

// Infos lists the searched types, in Type order
func (s *Session) Infos() (infos []TypeInfo) {
	for _, typ := range sessionScanTypesInOrder {
		m, ok := s.scans[typ]
		if !ok {
			continue
		}
		infos = append(infos, TypeInfo{Type: typ, Count: m.Count(), Status: s.status[typ]})
	}
	return
}

// firstScanTarget is one searched type during a first scan
type firstScanTarget struct {
	value   *scanner.Value
	matcher scanner.ChunkMatcher

	// hits[regionIndex] is sorted; guarded by firstScanState.mu once published
	hits     [][]uint64
	count    int
	disabled atomic.Bool
}

type firstScanState struct {
	mu      sync.Mutex
	targets []*firstScanTarget
	total   int
	limits  ResultLimits
}

// disable drops a type for the rest of the scan
func (st *firstScanState) disable(t *firstScanTarget) {
	t.disabled.Store(true)
	st.total -= t.count
	t.count = 0
	t.hits = nil
}

// publish stores the hits found in one region and enforces the limits
func (st *firstScanState) publish(regionIndex int, target int, hits []uint64) {
	if len(hits) == 0 {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()

	t := st.targets[target]
	if t.disabled.Load() {
		return
	}
	t.hits[regionIndex] = hits
	t.count += len(hits)
	st.total += len(hits)

	if t.count > st.limits.Hard {
		st.disable(t)
		return
	}
	for st.total > st.limits.Global {
		var biggest *firstScanTarget
		for _, c := range st.targets {
			if !c.disabled.Load() && (biggest == nil || c.count > biggest.count) {
				biggest = c
			}
		}
		if biggest == nil {
			return
		}
		st.disable(biggest)
	}
}

// FirstScan searches every value in a single pass over the process memory
func (s *Session) FirstScan(values []*scanner.Value, emulatorMode bool) (time.Duration, error) {
	s.Reset()

	if s.proc == nil || s.maps == nil {
		return 0, ErrSessionNotOpen
	}
	if len(values) == 0 {
		return 0, ErrNoValues
	}
	state := &firstScanState{limits: s.limits}
	seen := make(map[scanner.Type]bool, len(values))
	for _, v := range values {
		if v == nil {
			return 0, ErrNoValues
		}
		if seen[v.Type()] {
			return 0, fmt.Errorf("%w: %v", ErrDuplicateType, v.Type())
		}
		seen[v.Type()] = true
		matcher := scanner.NewChunkMatcher(*v)
		if matcher == nil {
			return 0, fmt.Errorf("%w: %v", ErrUnsupportedType, v.Type())
		}
		state.targets = append(state.targets, &firstScanTarget{value: v, matcher: matcher})
	}
	if !s.proc.Alive() {
		_ = s.Close()
		return 0, ErrProcessGone
	}

	if err := s.pause(); err != nil {
		return 0, fmt.Errorf("memscan: cannot pause the process: %w", err)
	}
	defer func() { _ = s.resume() }()

	regionLevel := REGION_ALL_RW
	if emulatorMode {
		regionLevel = REGION_ALL
	}

	st := time.Now()
	regions := RegionsOptimize(s.maps.Parse(regionLevel, emulatorMode))
	for _, t := range state.targets {
		t.hits = make([][]uint64, len(regions))
	}

	var wg sync.WaitGroup
	for index, region := range regions {
		if err := s.sem.Acquire(s.ctx, 1); err != nil {
			break // canceled
		}
		wg.Add(1)
		go func(index int, region Region) {
			defer s.sem.Release(1)
			defer wg.Done()
			s.scanRegion(index, region, state)
		}(index, region)
	}
	wg.Wait()

	if s.ctx.Err() != nil {
		return 0, ErrScanCanceled
	}

	// regions are in address order
	for _, t := range state.targets {
		m := newMemscanCaps(sessionInitialResultsCaps)
		m.Reset()
		m.proc = s.proc
		// Session.Cancel stops every following scan of every type
		m.ctx, m.cancel = s.ctx, s.cancel

		typ := t.value.Type()
		switch {
		case t.disabled.Load():
			s.status[typ] = StatusTruncated
		default:
			for i, hits := range t.hits {
				if err := m.results.AppendSlice(hits); err != nil {
					s.status[typ] = StatusTruncated
					m.results.Clear()
					break
				}
				t.hits[i] = nil // let the GC reclaim it as we go
			}
			if s.status[typ] != StatusTruncated {
				if m.results.Len() > s.limits.Soft {
					s.status[typ] = StatusParked
				}
				m.round = 1
			}
		}
		t.hits = nil
		s.scans[typ] = m
	}
	s.round = 1

	return time.Since(st), nil
}

func (s *Session) scanRegion(index int, region Region, state *firstScanState) {
	reader := region.Pipe(s.proc.PID)
	defer reader.Close()

	bufPtr := sessionChunkPool.Get().(*[]byte)
	defer sessionChunkPool.Put(bufPtr)
	buf := *bufPtr

	// hits found in this region, one list per target, allocated on first hit
	local := make([][]uint64, len(state.targets))
	collectors := make([]scanner.CollectorFunc, len(state.targets))
	for i := range collectors {
		i := i
		collectors[i] = func(offset int) bool {
			local[i] = append(local[i], region.Start+uint64(offset))
			return true
		}
	}

	offset := 0
	for s.ctx.Err() == nil {
		n, err := io.ReadFull(reader, buf)
		if n > 0 {
			chunk := buf[:n]
			active := false
			for i, t := range state.targets {
				if t.disabled.Load() {
					continue
				}
				active = true
				t.matcher.ScanChunk(chunk, offset, collectors[i])
				if len(local[i]) > state.limits.Hard {
					// this region alone already exceeds the limit
					state.mu.Lock()
					if !t.disabled.Load() {
						state.disable(t)
					}
					state.mu.Unlock()
					local[i] = nil
				}
			}
			if !active {
				return
			}
			offset += n
		}
		if err != nil {
			break // end of region, or unreadable memory: like Scanner, give up on it
		}
	}

	for i := range local {
		state.publish(index, i, local[i])
	}
}

// release frees the result buffers right away instead of waiting for the GC
func (m *Memscan) release() {
	if m.results != nil {
		m.results.Destroy()
	}
	if m.prevResults != nil {
		m.prevResults.Destroy()
	}
	m.Cancel()
}

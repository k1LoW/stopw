package stopw

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/xid"
)

var globalSpan = New()

// Start stopwatch
func Start(ids ...string) {
	globalSpan.Start(ids...)
}

// Stop stopwatch
func Stop(ids ...string) {
	globalSpan.Stop(ids...)
}

// StartAt start stopwatch by specifying the time
func StartAt(start time.Time, ids ...string) {
	globalSpan.StartAt(start, ids...)
}

// StopAt stop stopwatch by specifying the time
func StopAt(end time.Time, ids ...string) {
	globalSpan.StopAt(end, ids...)
}

// Reset measurement result
func Reset() {
	globalSpan.Reset()
}

// Result returns measurement result
func Result() *Span {
	return globalSpan.Result()
}

type Span struct {
	ID        string        `json:"id,omitempty"`
	StartedAt time.Time     `json:"started_at"`
	StoppedAt time.Time     `json:"stopped_at"`
	Elapsed   time.Duration `json:"elapsed"`
	Breakdown spans         `json:"breakdown,omitempty"`

	parent *Span
	mu     sync.RWMutex
}

type spans []*Span

// New return a new root Span
func New(ids ...string) *Span {
	switch len(ids) {
	case 0:
		return &Span{
			ID: xid.New().String(),
		}
	case 1:
		return &Span{
			ID: ids[0],
		}
	default:
		s := &Span{
			ID: ids[0],
		}
		return s.New(ids[1:]...)
	}
}

// New return a new breakdown span
func (s *Span) New(ids ...string) *Span {
	if len(ids) == 0 {
		s.mu.Lock()
		n := &Span{
			ID:     xid.New().String(),
			parent: s,
		}
		s.Breakdown = append(s.Breakdown, n)
		s.mu.Unlock()
		return n
	}
	var (
		n   *Span
		err error
	)
	n, err = s.findByIDs(ids[0])
	if err != nil {
		s.mu.Lock()
		n = &Span{
			ID:     ids[0],
			parent: s,
		}
		s.Breakdown = append(s.Breakdown, n)
		s.mu.Unlock()
	}
	if len(ids[1:]) == 0 {
		return n
	}
	return n.New(ids[1:]...)
}

// IDs returns ID list
func (s *Span) IDs() []string {
	var ids []string
	if s.parent != nil {
		ids = s.parent.IDs()
	}
	ids = append(ids, s.ID)

	return ids
}

// Start stopwatch of span
func (s *Span) Start(ids ...string) {
	start := time.Now()
	s.StartAt(start, ids...)
}

// Stop stopwatch of span
func (s *Span) Stop(ids ...string) {
	end := time.Now()
	s.StopAt(end, ids...)
}

// Reset measurement result of span
func (s *Span) Reset() {
	s.StartedAt = time.Time{}
	s.StoppedAt = time.Time{}
	s.Elapsed = 0
	s.parent = nil
	s.Breakdown = nil
}

// Result returns measurement result of span
func (s *Span) Result() *Span {
	return s.deepCopy()
}

// StartAt start stopwatch of span by specifying the time
func (s *Span) StartAt(start time.Time, ids ...string) {
	if len(ids) == 0 {
		s.Reset()
	}
	t := s.findOrNewByIDs(ids...)
	start = s.calcStartedAt(start)

	t.setStartedAt(start)
	t.setParentStartedAt(start)
}

func (s *Span) calcStartedAt(start time.Time) time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	startedAt := s.StartedAt
	if !startedAt.IsZero() && start.UnixNano() > startedAt.UnixNano() {
		start = startedAt
	}
	et := s.Breakdown.earliestStartedAt()
	if !et.IsZero() && et.UnixNano() < start.UnixNano() {
		start = et
	}
	return start
}

func (s *Span) setParentStartedAt(start time.Time) {
	if s.parent == nil {
		return
	}
	s.parent.mu.RLock()
	startedAt := s.parent.StartedAt
	s.parent.mu.RUnlock()
	if startedAt.IsZero() || startedAt.UnixNano() > start.UnixNano() {
		s.parent.setStartedAt(start)
	}
	s.parent.setParentStartedAt(start)
}

func (s *Span) setStartedAt(start time.Time) {
	s.mu.Lock()
	s.StartedAt = start
	s.mu.Unlock()
}

// StopAt stop stopwatch of span by specifying the time
func (s *Span) StopAt(end time.Time, ids ...string) {
	t, err := s.findByIDs(ids...)
	if err != nil {
		return
	}
	end = t.calcStoppedAt(end)
	t.setStoppedAt(end)
	t.setBreakdownStoppedAt(end)
	t.setParentStoppedAt(end)
}

func (s *Span) calcStoppedAt(end time.Time) time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stoppedAt := s.StoppedAt
	if end.UnixNano() < stoppedAt.UnixNano() {
		end = stoppedAt
	}
	lt := s.Breakdown.latestStoppedAt()
	if !lt.IsZero() && lt.UnixNano() > end.UnixNano() {
		end = lt
	}
	return end
}

func (s *Span) setBreakdownStoppedAt(end time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, b := range s.Breakdown {
		b.mu.RLock()
		stoppedAt := b.StoppedAt
		b.mu.RUnlock()
		if stoppedAt.IsZero() {
			b.setStoppedAt(end)
			b.setBreakdownStoppedAt(end)
		}
	}
}

func (s *Span) setParentStoppedAt(end time.Time) {
	if s.parent == nil {
		return
	}
	s.parent.mu.RLock()
	stoppedAt := s.parent.StoppedAt
	s.parent.mu.RUnlock()
	if stoppedAt.IsZero() || stoppedAt.UnixNano() < end.UnixNano() {
		s.parent.setStoppedAt(end)
	}
	s.parent.setParentStoppedAt(end)
}

func (s *Span) setStoppedAt(end time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.StartedAt.IsZero() {
		s.StartedAt = end
	}
	s.StoppedAt = end
	s.Elapsed = s.StoppedAt.Sub(s.StartedAt)
}

func (s *Span) findByIDs(ids ...string) (*Span, error) {
	if len(ids) == 0 {
		return s, nil
	}
	if s.parent == nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
	}
	for _, b := range s.Breakdown {
		if b.ID == ids[0] {
			if len(ids) > 1 {
				return b.findByIDs(ids[1:]...)
			}
			return b, nil
		}
	}
	return nil, fmt.Errorf("not found: %s", ids)
}

func (s *Span) findOrNewByIDs(ids ...string) *Span {
	t, err := s.findByIDs(ids...)
	if err != nil {
		return s.New(ids...)
	}
	return t
}

func (s *Span) deepCopy() *Span {
	cp := &Span{
		ID:        s.ID,
		StartedAt: s.StartedAt,
		StoppedAt: s.StoppedAt,
		Elapsed:   s.Elapsed,
	}
	for _, b := range s.Breakdown {
		bcp := b.deepCopy()
		bcp.parent = cp
		cp.Breakdown = append(cp.Breakdown, bcp)
	}
	return cp
}

func (ss spans) earliestStartedAt() time.Time {
	et := time.Time{}
	for _, s := range ss {
		s.mu.RLock()
		startedAt := s.StartedAt
		s.mu.RUnlock()
		if et.IsZero() || startedAt.UnixNano() < et.UnixNano() {
			et = startedAt
		}
	}
	return et
}

func (ss spans) latestStoppedAt() time.Time {
	lt := time.Time{}
	for _, s := range ss {
		s.mu.RLock()
		stoppedAt := s.StoppedAt
		s.mu.RUnlock()
		if stoppedAt.UnixNano() > lt.UnixNano() {
			lt = stoppedAt
		}
	}
	return lt
}

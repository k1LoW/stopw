package stopw

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rs/xid"
)

var globalSpan = New()

// Start stopwatch
func Start(ids ...any) *Span {
	return globalSpan.Start(ids...)
}

// Stop stopwatch
func Stop(ids ...any) {
	globalSpan.Stop(ids...)
}

// StartAt start stopwatch by specifying the time
func StartAt(start time.Time, ids ...any) *Span {
	return globalSpan.StartAt(start, ids...)
}

// StopAt stop stopwatch by specifying the time
func StopAt(end time.Time, ids ...any) {
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

// Copy stopwatch
func Copy() *Span {
	return globalSpan.Copy()
}

// Disable stopwatch
func Disable() *Span {
	return globalSpan.Disable()
}

// Enable stopwatch
func Enable() *Span {
	return globalSpan.Enable()
}

type Span struct {
	ID        any       `json:"id,omitempty"`
	StartedAt time.Time `json:"started_at"`
	StoppedAt time.Time `json:"stopped_at"`
	Breakdown spans     `json:"breakdown,omitempty"`

	disable bool
	parent  *Span
	mu      sync.RWMutex
}

type spans []*Span

// New return a new root Span
func New(ids ...any) *Span {
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
func (s *Span) New(ids ...any) *Span {
	if s.disable {
		return s
	}
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
func (s *Span) IDs() []any {
	if s.disable {
		return nil
	}
	var ids []any
	if s.parent != nil {
		ids = s.parent.IDs()
	}
	ids = append(ids, s.ID)

	return ids
}

// Start stopwatch of span
func (s *Span) Start(ids ...any) *Span {
	if s.disable {
		return s
	}
	start := time.Now()
	return s.StartAt(start, ids...)
}

// Stop stopwatch of span
func (s *Span) Stop(ids ...any) {
	if s.disable {
		return
	}
	end := time.Now()
	s.StopAt(end, ids...)
}

// StartAt start stopwatch of span by specifying the time
func (s *Span) StartAt(start time.Time, ids ...any) *Span {
	if s.disable {
		return s
	}
	if len(ids) == 0 {
		s.Reset()
	}
	t := s.findOrNewByIDs(ids...)
	start = t.calcStartedAt(start)
	t.setStartedAt(start)
	t.setParentStartedAt(start)
	return t
}

// StopAt stop stopwatch of span by specifying the time
func (s *Span) StopAt(end time.Time, ids ...any) {
	if s.disable {
		return
	}
	t, err := s.findByIDs(ids...)
	if err != nil {
		return
	}
	end = t.calcStoppedAt(end)
	t.setStoppedAt(end)
	t.setBreakdownStoppedAt(end)
	t.setParentStoppedAt(end)
}

// Reset measurement result of span
func (s *Span) Reset() {
	if s.disable {
		return
	}
	s.StartedAt = time.Time{}
	s.StoppedAt = time.Time{}
	s.parent = nil
	s.Breakdown = nil
}

func (s *Span) Elapsed() time.Duration {
	if s.disable {
		return 0
	}
	if s.StartedAt.IsZero() || s.StoppedAt.IsZero() {
		return 0
	}
	return s.StoppedAt.Sub(s.StartedAt)
}

// Disable stopwatch
func (s *Span) Disable() *Span {
	s.disable = true
	return s
}

// Enable stopwatch
func (s *Span) Enable() *Span {
	s.disable = false
	return s
}

func (s *Span) Repair() {
	for _, bs := range s.Breakdown {
		bs.parent = s
		bs.Repair()
	}
}

func (s *Span) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		ID        any           `json:"id,omitempty"`
		StartedAt time.Time     `json:"started_at"`
		StoppedAt time.Time     `json:"stopped_at"`
		Elapsed   time.Duration `json:"elapsed"`
		Breakdown spans         `json:"breakdown,omitempty"`
	}{
		ID:        s.ID,
		StartedAt: s.StartedAt,
		StoppedAt: s.StoppedAt,
		Elapsed:   s.Elapsed(),
		Breakdown: s.Breakdown,
	})
}

func (s *Span) calcStartedAt(start time.Time) time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	startedAt := s.StartedAt
	// startedAt != zero && startedAt < start
	if !startedAt.IsZero() && startedAt.Before(start) {
		start = startedAt
	}
	et := s.Breakdown.earliestStartedAt()
	// et != zero && et < start
	if !et.IsZero() && et.Before(start) {
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
	// startedAt == zero || startedAt > start
	if startedAt.IsZero() || startedAt.After(start) {
		s.parent.setStartedAt(start)
	}
	s.parent.setParentStartedAt(start)
}

func (s *Span) setStartedAt(start time.Time) {
	s.mu.Lock()
	s.StartedAt = start
	s.mu.Unlock()
}

func (s *Span) calcStoppedAt(end time.Time) time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stoppedAt := s.StoppedAt
	// end > stoppedAt
	if end.Before(stoppedAt) {
		end = stoppedAt
	}
	lt := s.Breakdown.latestStoppedAt()
	// lt != zero && lt > end
	if !lt.IsZero() && lt.After(end) {
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
	// stoppedAt == zero || stoppedAt < end
	if stoppedAt.IsZero() || stoppedAt.Before(end) {
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
}

func (s *Span) findByIDs(ids ...any) (*Span, error) {
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

func (s *Span) findOrNewByIDs(ids ...any) *Span {
	t, err := s.findByIDs(ids...)
	if err != nil {
		return s.New(ids...)
	}
	return t
}

// Result returns the result of the stopwatch.
func (s *Span) Result() *Span {
	if s.disable {
		return nil
	}
	return s
}

// Copy returns a copy of the stopwatch.
func (s *Span) Copy() *Span {
	cp := &Span{
		ID:        s.ID,
		StartedAt: s.StartedAt,
		StoppedAt: s.StoppedAt,
	}
	for _, b := range s.Breakdown {
		bcp := b.Copy()
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
		// et == zero || et > startedAt
		if et.IsZero() || et.After(startedAt) {
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
		// lt > stoppedAt
		if lt.After(stoppedAt) {
			lt = stoppedAt
		}
	}
	return lt
}

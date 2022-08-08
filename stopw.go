package stopw

import (
	"fmt"
	"sync"
	"time"
)

var globalMetric = New()

func Start(keys ...string) {
	globalMetric.Start(keys...)
}

func Stop(keys ...string) {
	globalMetric.Stop(keys...)
}

func StartAt(start time.Time, keys ...string) {
	globalMetric.StartAt(start, keys...)
}

func StopAt(end time.Time, keys ...string) {
	globalMetric.StopAt(end, keys...)
}

func Reset() {
	globalMetric.Reset()
}

func Result() *Metric {
	return globalMetric.Result()
}

type Metric struct {
	Key       string        `json:"key,omitempty"`
	StartedAt time.Time     `json:"started_at"`
	StoppedAt time.Time     `json:"stopped_at"`
	Elapsed   time.Duration `json:"elapsed"`
	Breakdown metrics       `json:"breakdown,omitempty"`

	parent *Metric
	mu     sync.RWMutex
}

type metrics []*Metric

// New return new root Metric
func New() *Metric {
	return &Metric{
		Key: "",
	}
}

func (m *Metric) New(keys ...string) *Metric {
	if len(keys) == 0 {
		return m
	}
	var (
		nm  *Metric
		err error
	)
	nm, err = m.findByKeys(keys[0])
	if err != nil {
		m.mu.Lock()
		nm = &Metric{
			Key:    keys[0],
			parent: m,
		}
		m.Breakdown = append(m.Breakdown, nm)
		m.mu.Unlock()
	}
	return nm.New(keys[1:]...)
}

func (m *Metric) Start(keys ...string) {
	start := time.Now()
	m.StartAt(start, keys...)
}

func (m *Metric) Stop(keys ...string) {
	end := time.Now()
	m.StopAt(end, keys...)
}

func (m *Metric) Reset() {
	m.StartedAt = time.Time{}
	m.StoppedAt = time.Time{}
	m.Elapsed = 0
	m.parent = nil
	m.Breakdown = nil
}

func (m *Metric) Result() *Metric {
	return m.deepCopy()
}

func (m *Metric) StartAt(start time.Time, keys ...string) {
	tm := m.findOrNewByKeys(keys...)
	start = m.calcStartedAt(start)

	tm.setStartedAt(start)
	tm.setParentStartedAt(start)
}

func (m *Metric) calcStartedAt(start time.Time) time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	startedAt := m.StartedAt
	if !startedAt.IsZero() && start.UnixNano() > startedAt.UnixNano() {
		start = startedAt
	}
	et := m.Breakdown.earliestStartedAt()
	if !et.IsZero() && et.UnixNano() < start.UnixNano() {
		start = et
	}
	return start
}

func (m *Metric) setParentStartedAt(start time.Time) {
	if m.parent == nil {
		return
	}
	m.parent.mu.RLock()
	startedAt := m.parent.StartedAt
	m.parent.mu.RUnlock()
	if startedAt.IsZero() || startedAt.UnixNano() > start.UnixNano() {
		m.parent.setStartedAt(start)
	}
	m.parent.setParentStartedAt(start)
}

func (m *Metric) setStartedAt(start time.Time) {
	m.mu.Lock()
	m.StartedAt = start
	m.mu.Unlock()
}

func (m *Metric) StopAt(end time.Time, keys ...string) {
	tm, err := m.findByKeys(keys...)
	if err != nil {
		return
	}
	end = tm.calcStoppedAt(end)
	tm.setStoppedAt(end)
	tm.setBreakdownStoppedAt(end)
	tm.setParentStoppedAt(end)
}

func (m *Metric) calcStoppedAt(end time.Time) time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stoppedAt := m.StoppedAt
	if end.UnixNano() < stoppedAt.UnixNano() {
		end = stoppedAt
	}
	lt := m.Breakdown.latestStoppedAt()
	if !lt.IsZero() && lt.UnixNano() > end.UnixNano() {
		end = lt
	}
	return end
}

func (m *Metric) setBreakdownStoppedAt(end time.Time) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, bm := range m.Breakdown {
		bm.mu.RLock()
		stoppedAt := bm.StoppedAt
		bm.mu.RUnlock()
		if stoppedAt.IsZero() {
			bm.setStoppedAt(end)
			bm.setBreakdownStoppedAt(end)
		}
	}
}

func (m *Metric) setParentStoppedAt(end time.Time) {
	if m.parent == nil {
		return
	}
	m.parent.mu.RLock()
	stoppedAt := m.parent.StoppedAt
	m.parent.mu.RUnlock()
	if stoppedAt.IsZero() || stoppedAt.UnixNano() < end.UnixNano() {
		m.parent.setStoppedAt(end)
	}
	m.parent.setParentStoppedAt(end)
}

func (m *Metric) setStoppedAt(end time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.StartedAt.IsZero() {
		m.StartedAt = end
	}
	m.StoppedAt = end
	m.Elapsed = m.StoppedAt.Sub(m.StartedAt)
}

func (m *Metric) findByKeys(keys ...string) (*Metric, error) {
	if len(keys) == 0 {
		return m, nil
	}
	if m.parent == nil {
		m.mu.RLock()
		defer m.mu.RUnlock()
	}
	for _, bm := range m.Breakdown {
		if bm.Key == keys[0] {
			if len(keys) > 1 {
				return bm.findByKeys(keys[1:]...)
			}
			return bm, nil
		}
	}
	return nil, fmt.Errorf("not found: %s", keys)
}

func (m *Metric) findOrNewByKeys(keys ...string) *Metric {
	t, err := m.findByKeys(keys...)
	if err != nil {
		return m.New(keys...)
	}
	return t
}

func (m *Metric) deepCopy() *Metric {
	cp := &Metric{
		Key:       m.Key,
		StartedAt: m.StartedAt,
		StoppedAt: m.StoppedAt,
		Elapsed:   m.Elapsed,
	}
	for _, bm := range m.Breakdown {
		bcp := bm.deepCopy()
		bcp.parent = cp
		cp.Breakdown = append(cp.Breakdown, bcp)
	}
	return cp
}

func (ms metrics) earliestStartedAt() time.Time {
	et := time.Time{}
	for _, m := range ms {
		m.mu.RLock()
		startedAt := m.StartedAt
		m.mu.RUnlock()
		if et.IsZero() || startedAt.UnixNano() < et.UnixNano() {
			et = startedAt
		}
	}
	return et
}

func (ms metrics) latestStoppedAt() time.Time {
	lt := time.Time{}
	for _, m := range ms {
		m.mu.RLock()
		stoppedAt := m.StoppedAt
		m.mu.RUnlock()
		if stoppedAt.UnixNano() > lt.UnixNano() {
			lt = stoppedAt
		}
	}
	return lt
}

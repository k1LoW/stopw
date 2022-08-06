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
	Breakdown []*Metric     `json:"breakdown,omitempty"`

	parent *Metric
	mu     *sync.RWMutex
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

// New return new root Metric
func New() *Metric {
	return &Metric{
		Key: "",
		mu:  &sync.RWMutex{},
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
			mu:     m.mu,
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
	tm.setStartedAt(start)
}

func (m *Metric) setStartedAt(start time.Time) {
	m.mu.Lock()
	if m.StartedAt.IsZero() {
		m.StartedAt = start
	} else {
		slided := false
		for _, bm := range m.Breakdown {
			if bm.StartedAt.UnixNano() < m.StartedAt.UnixNano() {
				slided = true
				start = bm.StartedAt
			}
		}
		if slided {
			m.StartedAt = start
		}
	}
	m.mu.Unlock()
	if m.parent != nil {
		m.parent.setStartedAt(start)
	}
}

func (m *Metric) StopAt(end time.Time, keys ...string) {
	tm, err := m.findByKeys(keys...)
	if err != nil {
		return
	}
	tm.setStoppedAt(end)
	if len(keys) == 0 {
		return
	}
	for _, bm := range tm.Breakdown {
		bm.StopAt(end, keys[1:]...)
	}
}

func (m *Metric) setStoppedAt(end time.Time) {
	m.mu.Lock()
	if m.StoppedAt.IsZero() {
		if m.StartedAt.IsZero() {
			m.StartedAt = end
		}
		m.StoppedAt = end
		m.Elapsed = m.StoppedAt.Sub(m.StartedAt)
	} else {
		slided := false
		for _, bm := range m.Breakdown {
			if bm.StoppedAt.UnixNano() > m.StoppedAt.UnixNano() {
				slided = true
				end = bm.StoppedAt
			}
		}
		if slided {
			m.StoppedAt = end
		}
	}
	m.mu.Unlock()
	if m.parent != nil {
		m.parent.setStoppedAt(end)
	}
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

package stopw

import (
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestNew(t *testing.T) {
	tests := []struct {
		keys     []string
		want     *Metric
		wantRoot *Metric
	}{
		{[]string{}, &Metric{}, &Metric{}},
		{[]string{"a"}, &Metric{Key: "a"}, &Metric{Breakdown: []*Metric{{Key: "a"}}}},
		{[]string{"a", "b"}, &Metric{Key: "b"}, &Metric{Breakdown: []*Metric{{Key: "a", Breakdown: []*Metric{{Key: "b"}}}}}},
	}
	for _, tt := range tests {
		root := New()
		got := root.New(tt.keys...)
		opts := cmp.Options{
			cmp.AllowUnexported(Metric{}),
			cmpopts.IgnoreFields(Metric{}, "parent", "Elapsed", "mu"),
		}
		if diff := cmp.Diff(got, tt.want, opts...); diff != "" {
			t.Errorf("%s", diff)
		}
		if diff := cmp.Diff(root, tt.wantRoot, opts); diff != "" {
			t.Errorf("%s", diff)
		}
	}
}

func TestStartStop(t *testing.T) {
	tests := []struct {
		st func() *Metric
	}{
		{
			func() *Metric {
				m := New()
				m.Start()
				m.Stop()
				return m
			},
		},
		{
			func() *Metric {
				m := New()
				m.Stop()
				return m
			},
		},
	}
	for _, tt := range tests {
		m := tt.st()
		if m.Elapsed < 0 {
			t.Errorf("invalid elapsed: %v", m.Elapsed)
		}
	}
}

func TestGlobal(t *testing.T) {
	Start()
	Stop()
	r := Result()
	if r.Elapsed <= 0 {
		t.Errorf("invalid elapsed: %v", r.Elapsed)
	}

	Reset()
	r2 := Result()
	if r.Elapsed <= 0 {
		t.Errorf("invalid elapsed: %v", r.Elapsed)
	}
	if r2.Elapsed != 0 {
		t.Errorf("invalid elapsed: %v", r2.Elapsed)
	}
}

func TestNest(t *testing.T) {
	m := New()
	m.Start()
	m.Start("sub A")
	m.Start("sub B")
	m.Start("sub A", "sub sub a")
	m.Stop("sub A", "sub sub a")
	m.Stop("sub A")
	m.Start("sub B")
	m.Stop()

	if want := 2; len(m.Breakdown) != want {
		t.Errorf("got %v\nwant %v", len(m.Breakdown), want)
	}
}

func TestConcurrent(t *testing.T) {
	Start()
	for i := 0; i < 100; i++ {
		go func(i int) {
			Start(strconv.Itoa(i))
			Stop(strconv.Itoa(i))
		}(i)
	}
	Stop()
}

func TestRootAutoStartStop(t *testing.T) {
	m := New()
	m.Start("first")
	m.Stop("first")
	m.Start("second")
	m.Stop("second")

	root := m.Result()

	fr, err := m.findByKeys("first")
	if err != nil {
		t.Fatal(err)
	}
	sr, err := m.findByKeys("second")
	if err != nil {
		t.Fatal(err)
	}

	if root.StartedAt.UnixNano() != fr.StartedAt.UnixNano() {
		t.Errorf("StartedAt got %v %v\nwant same", root.StartedAt, fr.StartedAt)
	}

	if root.StoppedAt.UnixNano() != sr.StoppedAt.UnixNano() {
		t.Errorf("StoppedAt got %v %v\nwant same", root.StoppedAt, sr.StoppedAt)
	}
}

func TestStartAt(t *testing.T) {
	start := time.Now()
	tests := []struct {
		keys []string
		want *Metric
	}{
		{[]string{}, &Metric{StartedAt: start}},
		{[]string{"a"}, &Metric{StartedAt: start, Breakdown: []*Metric{{Key: "a", StartedAt: start}}}},
		{[]string{"a", "b"}, &Metric{StartedAt: start, Breakdown: []*Metric{{Key: "a", StartedAt: start, Breakdown: []*Metric{{Key: "b", StartedAt: start}}}}}},
	}
	for _, tt := range tests {
		m := New()
		m.StartAt(start, tt.keys...)
		opts := cmp.Options{
			cmp.AllowUnexported(Metric{}),
			cmpopts.IgnoreFields(Metric{}, "parent", "Elapsed", "mu"),
		}
		if diff := cmp.Diff(m, tt.want, opts...); diff != "" {
			t.Errorf("%s", diff)
		}
	}
}

func TestStopAt(t *testing.T) {
	start := time.Now()
	end := time.Now().Add(1 * time.Second)
	tests := []struct {
		keys []string
		want *Metric
	}{
		{[]string{}, &Metric{StartedAt: start, StoppedAt: end}},
		{[]string{"a"}, &Metric{StartedAt: start, StoppedAt: end, Breakdown: []*Metric{{Key: "a", StartedAt: start, StoppedAt: end}}}},
		{[]string{"a", "b"}, &Metric{StartedAt: start, StoppedAt: end, Breakdown: []*Metric{{Key: "a", StartedAt: start, StoppedAt: end, Breakdown: []*Metric{{Key: "b", StartedAt: start, StoppedAt: end}}}}}},
	}
	for _, tt := range tests {
		m := New()
		m.StartAt(start, tt.keys...)
		m.StopAt(end, tt.keys...)
		opts := cmp.Options{
			cmp.AllowUnexported(Metric{}),
			cmpopts.IgnoreFields(Metric{}, "parent", "Elapsed", "mu"),
		}
		if diff := cmp.Diff(m, tt.want, opts...); diff != "" {
			t.Errorf("%s", diff)
		}
	}
}

package stopw

import (
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/rs/xid"
)

const testID = "generated"

func TestNew(t *testing.T) {
	tests := []struct {
		ids  []string
		want *Span
	}{
		{[]string{}, &Span{ID: testID}},
		{[]string{"a"}, &Span{ID: "a"}},
		{[]string{"a", "b"}, &Span{ID: "b"}},
	}
	for _, tt := range tests {
		got := New(tt.ids...)
		opts := cmp.Options{
			cmp.AllowUnexported(Span{}),
			cmpopts.IgnoreFields(Span{}, "parent", "Elapsed", "mu"),
		}
		convertID(got)
		if diff := cmp.Diff(got, tt.want, opts...); diff != "" {
			t.Errorf("%s", diff)
		}
	}
}

func TestNestedNew(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	n := rand.Intn(100)
	var s *Span
	for i := 0; i < n; i++ {
		if s == nil {
			s = New()
		} else {
			s = s.New()
		}
	}
	got := len(s.IDs())
	if got != n {
		t.Errorf("got %v\nwant %v", got, n)
	}
}

func TestStartStop(t *testing.T) {
	tests := []struct {
		st func() *Span
	}{
		{
			func() *Span {
				s := New()
				s.Start()
				s.Stop()
				return s
			},
		},
		{
			func() *Span {
				s := New()
				s.Stop()
				return s
			},
		},
	}
	for _, tt := range tests {
		s := tt.st()
		if s.Elapsed < 0 {
			t.Errorf("invalid elapsed: %v", s.Elapsed)
		}
		validate(t, s)
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
	validate(t, r)
	if r2.Elapsed != 0 {
		t.Errorf("invalid elapsed: %v", r2.Elapsed)
	}
}

func TestNest(t *testing.T) {
	s := New()
	s.Start()
	s.Start("sub A")
	s.Start("sub B")
	s.Start("sub A", "sub sub a")
	s.Stop("sub A", "sub sub a")
	s.Stop("sub A")
	s.Start("sub B")
	s.Stop()

	if want := 2; len(s.Breakdown) != want {
		t.Errorf("got %v\nwant %v", len(s.Breakdown), want)
	}
	validate(t, s)
}

func TestConcurrent(t *testing.T) {
	Start()
	wg := &sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			Start(strconv.Itoa(i))
			Stop(strconv.Itoa(i))
			wg.Done()
		}(i)
	}
	wg.Wait()
	Stop()
}

func TestAutoStartStopRoot(t *testing.T) {
	s := New()
	s.Start("first")
	s.Stop("first")
	s.Start("second")
	s.Stop("second")

	root := s.Result()

	fr, err := s.findByIDs("first")
	if err != nil {
		t.Fatal(err)
	}
	sr, err := s.findByIDs("second")
	if err != nil {
		t.Fatal(err)
	}

	if root.StartedAt.UnixNano() != fr.StartedAt.UnixNano() {
		t.Errorf("got %v and %v\nwant same", root.StartedAt, fr.StartedAt)
	}

	if root.StoppedAt.UnixNano() != sr.StoppedAt.UnixNano() {
		t.Errorf("got %v and %v\nwant same", root.StoppedAt, sr.StoppedAt)
	}

	validate(t, root)
}

func TestAutoStopBreakdown(t *testing.T) {
	s := New()
	s.Start("first")
	s.Start("first", "second")
	s.Start("third")
	s.Stop("third")
	time.Sleep(1 * time.Microsecond)
	s.Stop()

	root := s.Result()

	fr, err := s.findByIDs("first")
	if err != nil {
		t.Fatal(err)
	}
	sr, err := s.findByIDs("first", "second")
	if err != nil {
		t.Fatal(err)
	}
	tr, err := s.findByIDs("third")
	if err != nil {
		t.Fatal(err)
	}

	if root.StoppedAt.UnixNano() != fr.StoppedAt.UnixNano() {
		t.Errorf("got %v and %v\nwant same", root.StoppedAt, fr.StoppedAt)
	}

	if root.StoppedAt.UnixNano() != sr.StoppedAt.UnixNano() {
		t.Errorf("got %v and %v\nwant same", root.StoppedAt, sr.StoppedAt)
	}

	if root.StoppedAt.UnixNano() == tr.StoppedAt.UnixNano() {
		t.Errorf("got %v and %v\nwant different", root.StoppedAt, tr.StoppedAt)
	}

	validate(t, root)
}

func TestParentStartTimeSlidesToEarliestEimeInBreakdown(t *testing.T) {
	earliest := time.Now().Add(-1 * time.Minute)

	s := New()
	s.Start("first")
	s.Stop("first")
	s.Start("second")
	s.Stop("second")

	s.StartAt(earliest, "third")
	s.Stop("third")

	root := s.Result()
	if root.StartedAt.UnixNano() != earliest.UnixNano() {
		t.Errorf("got %v and %v\nwant same", root.StartedAt, earliest)
	}

	validate(t, root)
}

func TestStartAt(t *testing.T) {
	start := time.Now()
	tests := []struct {
		ids  []string
		want *Span
	}{
		{[]string{}, &Span{ID: testID, StartedAt: start}},
		{[]string{"a"}, &Span{ID: testID, StartedAt: start, Breakdown: []*Span{{ID: "a", StartedAt: start}}}},
		{[]string{"a", "b"}, &Span{ID: testID, StartedAt: start, Breakdown: []*Span{{ID: "a", StartedAt: start, Breakdown: []*Span{{ID: "b", StartedAt: start}}}}}},
	}
	for _, tt := range tests {
		s := New()
		s.StartAt(start, tt.ids...)
		opts := cmp.Options{
			cmp.AllowUnexported(Span{}),
			cmpopts.IgnoreFields(Span{}, "parent", "Elapsed", "mu"),
		}
		convertID(s)
		if diff := cmp.Diff(s, tt.want, opts...); diff != "" {
			t.Errorf("%s", diff)
		}
	}
}

func TestStopAt(t *testing.T) {
	start := time.Now()
	end := time.Now().Add(1 * time.Second)
	tests := []struct {
		ids  []string
		want *Span
	}{
		{[]string{}, &Span{ID: testID, StartedAt: start, StoppedAt: end}},
		{[]string{"a"}, &Span{ID: testID, StartedAt: start, StoppedAt: end, Breakdown: []*Span{{ID: "a", StartedAt: start, StoppedAt: end}}}},
		{[]string{"a", "b"}, &Span{ID: testID, StartedAt: start, StoppedAt: end, Breakdown: []*Span{{ID: "a", StartedAt: start, StoppedAt: end, Breakdown: []*Span{{ID: "b", StartedAt: start, StoppedAt: end}}}}}},
	}
	for _, tt := range tests {
		s := New()
		s.StartAt(start, tt.ids...)
		s.StopAt(end, tt.ids...)
		opts := cmp.Options{
			cmp.AllowUnexported(Span{}),
			cmpopts.IgnoreFields(Span{}, "parent", "Elapsed", "mu"),
		}
		convertID(s)
		if diff := cmp.Diff(s, tt.want, opts...); diff != "" {
			t.Errorf("%s", diff)
		}
		validate(t, s)
	}
}

func TestIDs(t *testing.T) {
	tests := []struct {
		ids  []string
		want []string
	}{
		{[]string{"a", "b", "c"}, []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		s := New(tt.ids...)
		got := s.IDs()
		if diff := cmp.Diff(got, tt.want, nil); diff != "" {
			t.Errorf("%s", diff)
		}
	}
}

func convertID(s *Span) {
	if _, err := xid.FromString(s.ID); err == nil {
		s.ID = testID
	}
	for _, b := range s.Breakdown {
		convertID(b)
	}
}

func validate(t *testing.T, s *Span) {
	t.Helper()
	if s.StartedAt.IsZero() {
		t.Errorf("startedAt is zero: %s", s.ID)
	}
	if s.StoppedAt.IsZero() {
		t.Errorf("stoppedAt is zero: %s", s.ID)
	}
	if s.StartedAt.UnixNano() > s.StoppedAt.UnixNano() {
		t.Errorf("startedAt > stoppedAt: %s, %s", s.StartedAt, s.StoppedAt)
	}
	for _, b := range s.Breakdown {
		validate(t, b)
		if s.StartedAt.UnixNano() > b.StartedAt.UnixNano() {
			t.Errorf("startedAt > breakdown startedAt: %s, %s", s.StartedAt, b.StartedAt)
		}
		if s.StoppedAt.UnixNano() < b.StoppedAt.UnixNano() {
			t.Errorf("stoppedAt > breakdown stoppedAt: %s, %s", s.StoppedAt, b.StoppedAt)
		}
	}
}

package stopw

import (
	"encoding/json"
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
		ids  []interface{}
		want *Span
	}{
		{[]interface{}{}, &Span{ID: testID}},
		{[]interface{}{"a"}, &Span{ID: "a"}},
		{[]interface{}{"a", 2}, &Span{ID: 2}},
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
	n := rand.Intn(100) + 1
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
		{
			func() *Span {
				s := New()
				s.Start().Stop()
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
	s.Start("sub B", "sub sub b")
	s.Stop("sub B")
	s.Stop()

	if want := 2; len(s.Breakdown) != want {
		t.Errorf("got %v\nwant %v", len(s.Breakdown), want)
	}

	if _, err := s.findByIDs("sub A"); err != nil {
		t.Error(err)
	}
	if _, err := s.findByIDs("sub B"); err != nil {
		t.Error(err)
	}
	if _, err := s.findByIDs("sub A", "sub sub a"); err != nil {
		t.Error(err)
	}
	if _, err := s.findByIDs("sub B", "sub sub b"); err != nil {
		t.Error(err)
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
	time.Sleep(1 * time.Nanosecond)
	s.Start("first")
	time.Sleep(1 * time.Nanosecond)
	s.Stop("first")
	time.Sleep(1 * time.Nanosecond)
	s.Start("second")
	time.Sleep(1 * time.Nanosecond)
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

func TestStart(t *testing.T) {
	t.Run("Start time is recorded separately", func(t *testing.T) {
		t.Parallel()
		s := New()
		s.Start()
		sub := s.New("a")
		s.Start("a")
		s.Stop()

		if s.StartedAt.UnixNano() == sub.StartedAt.UnixNano() {
			t.Errorf("got %v want different", sub.StartedAt)
		}
	})

	t.Run("When the sub-span starts, if the parent-span has not started, it will be at the same time", func(t *testing.T) {
		t.Parallel()
		s := New()
		sub := s.New("a")
		s.Start("a")
		s.Stop()

		if s.StartedAt.UnixNano() != sub.StartedAt.UnixNano() {
			t.Errorf("got %v and %v\nwant same", s.StartedAt, sub.StartedAt)
		}
	})
}

func TestStartAt(t *testing.T) {
	start := time.Now()
	tests := []struct {
		ids  []interface{}
		want *Span
	}{
		{[]interface{}{}, &Span{ID: testID, StartedAt: start}},
		{[]interface{}{"a"}, &Span{ID: testID, StartedAt: start, Breakdown: []*Span{{ID: "a", StartedAt: start}}}},
		{[]interface{}{"a", "b"}, &Span{ID: testID, StartedAt: start, Breakdown: []*Span{{ID: "a", StartedAt: start, Breakdown: []*Span{{ID: "b", StartedAt: start}}}}}},
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
		ids  []interface{}
		want *Span
	}{
		{[]interface{}{}, &Span{ID: testID, StartedAt: start, StoppedAt: end}},
		{[]interface{}{"a"}, &Span{ID: testID, StartedAt: start, StoppedAt: end, Breakdown: []*Span{{ID: "a", StartedAt: start, StoppedAt: end}}}},
		{[]interface{}{"a", "b"}, &Span{ID: testID, StartedAt: start, StoppedAt: end, Breakdown: []*Span{{ID: "a", StartedAt: start, StoppedAt: end, Breakdown: []*Span{{ID: "b", StartedAt: start, StoppedAt: end}}}}}},
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
		ids  []interface{}
		want []interface{}
	}{
		{[]interface{}{"a", "b", "c"}, []interface{}{"a", "b", "c"}},
	}
	for _, tt := range tests {
		s := New(tt.ids...)
		got := s.IDs()
		if diff := cmp.Diff(got, tt.want, nil); diff != "" {
			t.Errorf("%s", diff)
		}
	}
}

func TestDisable(t *testing.T) {
	s := New()
	s.Disable()
	s.Start("first")
	s.Stop("first")
	s.Start("second", "third")
	s.Stop("second", "third")
	got := s.Result()
	if got != nil {
		t.Errorf("got %v\nwant %v", got, nil)
	}
	if !s.StartedAt.IsZero() {
		t.Errorf("got %v\nwant %v", s.StartedAt, time.Time{})
	}
	if !s.StoppedAt.IsZero() {
		t.Errorf("got %v\nwant %v", s.StoppedAt, time.Time{})
	}
	if len(s.Breakdown) != 0 {
		t.Errorf("got %v\nwant %v", len(s.Breakdown), 0)
	}
}

func TestRepair(t *testing.T) {
	s1 := New()
	s1.Start("first")
	s1.Stop("first")
	s1.Start("second", "third")
	s1.Stop("second", "third")
	b, err := json.Marshal(s1)
	if err != nil {
		t.Fatal(err)
	}
	var s2 *Span
	if err := json.Unmarshal(b, &s2); err != nil {
		t.Fatal(err)
	}
	s2.Repair()
	opts := cmp.Options{
		cmp.AllowUnexported(Span{}),
		cmpopts.IgnoreFields(Span{}, "mu"),
	}
	if diff := cmp.Diff(s1, s2, opts...); diff != "" {
		t.Errorf("%s", diff)
	}
}

func convertID(s *Span) {
	str, ok := s.ID.(string)
	if !ok {
		return
	}
	if _, err := xid.FromString(str); err == nil {
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

package hashring

import (
	"fmt"
	"io"
	"math"
	"math/rand"
	"sort"
	"strings"
	"testing"

	"github.com/gobwas/avl"
)

func ExampleRing() {
	var ring Ring
	ring.Insert(StringItem("server01"), 1)
	ring.Insert(StringItem("server02"), 1)
	ring.Insert(StringItem("server03"), 1)
	ring.Insert(StringItem("server04"), 1)

	fmt.Println(ring.Get(StringItem("user01")))
	fmt.Println(ring.Get(StringItem("user02")))
	fmt.Println(ring.Get(StringItem("user03")))
	fmt.Println(ring.Get(StringItem("user04")))

	// Output:
	// server04
	// server04
	// server02
	// server01
}

type distCase struct {
	name    string
	ring    map[string]float64
	dist    map[string]float64
	prec    float64
	actions []ringAction
}

var distCases = []distCase{
	{
		name: "single",
		ring: map[string]float64{
			"foo": 1,
		},
		dist: map[string]float64{
			"foo": 100,
		},
		prec: 0,
	},
	{
		name: "double",
		ring: map[string]float64{
			"foo": 1,
			"bar": 1,
		},
		dist: map[string]float64{
			"foo": 50,
			"bar": 50,
		},
		prec: 1,
	},
	{
		ring: map[string]float64{
			"foo": 1,
			"bar": 2,
		},
		dist: map[string]float64{
			"foo": 33,
			"bar": 66,
		},
		prec: 4.5,
	},
	{
		ring: map[string]float64{
			"foo": 1,
			"bar": 1,
			"baz": 3,
		},
		dist: map[string]float64{
			"foo": 20,
			"bar": 20,
			"baz": 60,
		},
		prec: 4,
	},
	{
		ring: map[string]float64{
			"foo": 1,
			"bar": 1,
			"baz": 1,
			"baq": 2,
		},
		dist: map[string]float64{
			"foo": 20,
			"bar": 20,
			"baz": 20,
			"baq": 40,
		},
		prec: 4,
	},
	{
		ring: map[string]float64{
			"foo": 1,
			"bar": 2,
		},
		actions: []ringAction{
			updateItem("foo", 3),
		},
		dist: map[string]float64{
			"foo": 60,
			"bar": 40,
		},
		prec: 4,
	},
	{
		ring: map[string]float64{
			"foo": 1,
			"bar": 2,
			"baz": 3,
		},
		actions: []ringAction{
			deleteItem("bar"),
		},
		dist: map[string]float64{
			"foo": 25,
			"baz": 75,
		},
		prec: 4.5,
	},
}

func TestRingGet(t *testing.T) {
	for _, test := range distCases {
		t.Run(test.name, func(t *testing.T) {
			r := makeRing(t, test.ring, test.actions...)
			act := getDistribution(t, r, 1e6)
			assertDistribution(t, act, test.dist, test.prec)
		})
	}
}

func TestRingGetEmpty(t *testing.T) {
	var r Ring
	if item := r.Get(IntItem(42)); item != nil {
		t.Fatalf("unexpected item from empty ring")
	}
}

// TestRingGetRelocation tests that after deletion of any server only 1/N of
// objects get relocated to other server(s).
func TestRingGetRelocation(t *testing.T) {
	const precFactor = 1.1

	for _, test := range []struct {
		name string
		ring map[string]float64
		prec float64
	}{
		{
			name: "two",
			ring: map[string]float64{
				"foo": 1,
				"bar": 1,
			},
		},
		{
			name: "three",
			ring: map[string]float64{
				"foo": 1,
				"bar": 1,
				"baz": 1,
			},
		},
	} {
		keys := make([]string, 0, len(test.ring))
		for k := range test.ring {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, del := range keys {
			t.Run(test.name+"/delete/"+del, func(t *testing.T) {
				r := makeRing(t, test.ring)

				prev := getDistribution(t, r, 1e6)
				if err := r.Delete(StringItem(del)); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				next := getDistribution(t, r, 1e6)

				var diff float64
				for key, a := range next {
					b := prev[key]
					d := a - b
					if d < 0 {
						t.Fatalf("unexpected negative diff for key %q", key)
					}
					diff += d
					delete(prev, key)
				}
				var deleted string
				for key := range prev {
					if deleted != "" {
						t.Fatalf("too many deleted keys")
					}
					deleted = key
				}
				if deleted != del {
					t.Fatalf("unexpected deleted key: %q", deleted)
				}

				act := diff / 100
				exp := precFactor * (1 / float64(len(test.ring)))
				if act > exp {
					t.Fatalf(
						"unexpected relocation size: %.2f; want at most %.2f",
						act, exp,
					)
				}

				//assertDistribution(t, diff, test.diff, test.prec)
			})
		}
	}
}

func TestRingInsertDuplicate(t *testing.T) {
	var r Ring
	x := StringItem("foo")
	if err := r.Insert(x, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := r.Insert(x, 2); err == nil {
		t.Fatalf("want error; got nothing")
	}
}

func TestRingDeleteNotExisting(t *testing.T) {
	var r Ring
	x := StringItem("foo")
	if err := r.Delete(x); err == nil {
		t.Fatalf("want error; got nothing")
	}
}

func TestRingUpdateNotExisting(t *testing.T) {
	var r Ring
	x := StringItem("foo")
	if err := r.Update(x, 42); err == nil {
		t.Fatalf("want error; got nothing")
	}
}

func TestRingDistribution(t *testing.T) {
	for _, test := range distCases {
		t.Run(test.name, func(t *testing.T) {
			r := makeRing(t, test.ring, test.actions...)
			act := make(map[string]float64)
			keyDistribution(r, func(x Item, v uint64) {
				d := float64(v) / float64(math.MaxUint64) * 100
				act[string(x.(StringItem))] = d
			})
			assertDistribution(t, act, test.dist, test.prec)
		})
	}
}

func TestRingCollisions(t *testing.T) {
	// Skip if no `-tags debug` was given.
	if !debug {
		t.Skip("no debug buildtag")
	}

	for _, test := range []struct {
		name   string
		digest map[digestArgs]uint64
		rings  [][]ringAction
	}{
		{
			name: "simple",
			digest: map[digestArgs]uint64{
				digestCall("bar", 0, 0):   42,
				digestCall("foo", 0, 159): 42,
			},
			rings: [][]ringAction{
				{
					insertItem("bar", 10),
					insertItem("foo", 10),
				},
			},
		},
		{
			// Case when foo and bar collide at value 42, and then, next
			// generation of bar collides with some other foo's point value.
			// After deletion of foo's 42 bar should be reset to initial
			// generation.
			name: "two generations",
			digest: map[digestArgs]uint64{
				digestCall("foo", 0, 1):   99,
				digestCall("foo", 0, 159): 42,

				// Collides with foo 0:159 (when foo has equal weight to bar).
				digestCall("bar", 0, 0): 42,

				// Next gen 0th bar point: collides with foo 0:1 (only when
				// there is full set of foo's points -- first gen 0th bar's
				// point collided with first gen 159th foo's point).
				digestCall("bar", 1, 0): 99,
			},
			rings: [][]ringAction{
				{
					insertItem("foo", 1),
					insertItem("bar", 1),
					updateItem("bar", 10), // Removes foo's 159 point due to weight change.
				},
				{
					insertItem("bar", 10),
					insertItem("foo", 1),
				},
			},
		},
		{
			name: "xxx",
			digest: map[digestArgs]uint64{
				digestCall("foo", 0, 0):   1,
				digestCall("foo", 0, 159): 2,
				digestCall("foo", 1, 159): 3,

				digestCall("bar", 0, 0): 1,
				digestCall("bar", 1, 0): 2,
				digestCall("bar", 2, 0): 3,
			},
			rings: [][]ringAction{
				{
					insertItem("foo", 1),
					insertItem("bar", 1),
					updateItem("bar", 1.1), // Removes foo's 159 point due to weight change.
				},
				{
					insertItem("bar", 1.1),
					insertItem("foo", 1),
				},
			},
		},
		{
			digest: map[digestArgs]uint64{
				digestCall("bar", 0, 0):   42,
				digestCall("foo", 0, 159): 42,
			},
			rings: [][]ringAction{
				{
					insertItem("bar", 10),
					insertItem("foo", 1),
					updateItem("foo", 10),
					updateItem("foo", 1),
				},
				{
					insertItem("bar", 10),
					insertItem("foo", 1),
				},
			},
		},
		{
			digest: map[digestArgs]uint64{
				digestCall("foo", 0, 15): 42,
				digestCall("bar", 0, 15): 42,
			},
			rings: permActions(
				insertItem("foo", 1),
				insertItem("bar", 1),
			),
		},
		{
			digest: map[digestArgs]uint64{
				digestCall("foo", 0, 15): 42,
				digestCall("bar", 0, 15): 42,
				digestCall("baz", 0, 15): 42,
			},
			rings: permActions(
				insertItem("foo", 1),
				insertItem("bar", 1),
				insertItem("baz", 1),
			),
		},
		{
			digest: map[digestArgs]uint64{
				digestCall("foo", 0, 15): 42,
				digestCall("bar", 0, 15): 42,
				digestCall("baz", 0, 15): 42,
				digestCall("baq", 0, 15): 42,
			},
			rings: permActions(
				insertItem("foo", 1),
				insertItem("bar", 1),
				insertItem("baz", 1),
				insertItem("baq", 1),
			),
		},
		{
			digest: map[digestArgs]uint64{
				digestCall("foo", 0, 15): 42,
				digestCall("bar", 0, 15): 42,
				digestCall("baz", 0, 15): 42,
			},
			rings: [][]ringAction{
				{
					insertItem("foo", 1),
					insertItem("baz", 1),
					insertItem("bar", 1),
					deleteItem("baz"),
				},
				{
					insertItem("foo", 1),
					insertItem("bar", 1),
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupDigest(t, test.digest)

			rings := make([]Ring, len(test.rings))
			for i, actions := range test.rings {
				fmt.Println(i, actions)
				setupRingTrace(&rings[i])
				applyActions(t, &rings[i], actions...)
			}
			for i := 1; i < len(rings); i++ {
				r0 := &rings[i-1]
				r1 := &rings[i-0]
				assertRingsEqual(t, fmt.Sprintf("%d ?= %d", i-1, i-0), r0, r1)
			}
		})
	}
}

func applyActions(t testing.TB, r *Ring, actions ...ringAction) {
	for _, a := range actions {
		if err := a.apply(r); err != nil {
			t.Fatalf("can't apply action %s: %v", a, err)
		}
	}
}

func makeRing(t testing.TB, keys map[string]float64, actions ...ringAction) *Ring {
	var r Ring
	for key, weight := range keys {
		err := r.Insert(StringItem(key), weight)
		if err != nil {
			t.Fatal(err)
		}
	}
	applyActions(t, &r, actions...)
	return &r
}

func keyDistribution(r *Ring, fn func(Item, uint64)) {
	r.ring.RLock()
	defer r.ring.RUnlock()
	var (
		prev uint64

		temp  = map[uint64]uint64{}
		index = map[uint64]Item{}
	)
	r.root.InOrder(func(x avl.Item) bool {
		p := x.(*point)
		d := p.value - prev
		prev = p.value
		temp[p.bucket.id] += d
		index[p.bucket.id] = p.bucket.item
		return true
	})

	// All objects greater than r.root.Max() (prev hash value) falls into
	// r.root.Min() bucket.
	min := r.root.Min().(*point).bucket.id
	temp[min] += math.MaxUint64 - prev

	for id, dist := range temp {
		item := index[id]
		fn(item, dist)
	}
}

func assertDistribution(t testing.TB, act, exp map[string]float64, prec float64) {
	for key, act := range act {
		exp := exp[key]
		diff := act - exp
		if math.Abs(diff) > prec {
			t.Errorf(
				"unexpected distribution for %q key: %.2f; want %.2f "+
					"(±%.2f%%, diff is %+.2f%%))",
				key, act, exp, prec, diff,
			)
		}
	}
}

func getDistribution(t testing.TB, r *Ring, numGet int) map[string]float64 {
	tmp := make(map[string]int)
	act := make(map[string]float64)
	for i := 0; i < numGet; i++ {
		n := rand.Intn(maxInt)
		item := r.Get(IntItem(n))
		if item == nil {
			t.Fatalf("unexpected empty item")
		}
		tmp[string(item.(StringItem))] += 1
	}
	for key, num := range tmp {
		act[key] = float64(num) / float64(numGet) * 100
	}
	return act
}

type ringAction interface {
	apply(*Ring) error
}

func permActions(actions ...ringAction) (ret [][]ringAction) {
	var f func(x ringAction, xs []ringAction) [][]ringAction
	f = func(x ringAction, xs []ringAction) (ret [][]ringAction) {
		if len(xs) == 0 {
			return [][]ringAction{{x}}
		}
		for _, ps := range f(xs[0], xs[1:]) {
			// Append current action to the end of received actions.
			// Below we will swap it with every element in the slice.
			ps = append(ps, x)

			last := len(ps) - 1
			for i := 0; i < len(ps); i++ {
				cp := append(([]ringAction)(nil), ps...)
				cp[i], cp[last] = cp[last], cp[i]
				ret = append(ret, cp)
			}
		}
		return ret
	}
	return f(actions[0], actions[1:])
}

type insertRingAction struct {
	s string
	w float64
}

func insertItem(s string, w float64) *insertRingAction {
	return &insertRingAction{
		s: s,
		w: w,
	}
}

func (i insertRingAction) String() string {
	return fmt.Sprintf("insert %s~%.2f", i.s, i.w)
}

func (i insertRingAction) apply(r *Ring) error {
	return r.Insert(StringItem(i.s), i.w)
}

type updateRingAction struct {
	s string
	w float64
}

func updateItem(s string, w float64) *updateRingAction {
	return &updateRingAction{
		s: s,
		w: w,
	}
}

func (i updateRingAction) String() string {
	return fmt.Sprintf("update %s@%.2f", i.s, i.w)
}

func (i updateRingAction) apply(r *Ring) error {
	return r.Update(StringItem(i.s), i.w)
}

type deleteRingAction struct {
	s string
}

func deleteItem(s string) *deleteRingAction {
	return &deleteRingAction{s}
}

func (d deleteRingAction) String() string {
	return fmt.Sprintf("delete %s", d.s)
}

func (d deleteRingAction) apply(r *Ring) error {
	return r.Delete(StringItem(d.s))
}

func ringPoints(r *Ring) (ps []*point) {
	r.root.InOrder(func(x avl.Item) bool {
		ps = append(ps, x.(*point))
		return true
	})
	return ps
}

func assertRingsEqual(t *testing.T, spec string, r0, r1 *Ring) {
	ps0 := ringPoints(r0)
	ps1 := ringPoints(r1)
	if n0, n1 := len(ps0), len(ps1); n0 != n1 {
		t.Fatalf("%s: sizes are not equal: %d vs %d", spec, n0, n1)
	}
	for i, p0 := range ps0 {
		p1 := ps1[i]
		if p0.value != p1.value {
			t.Fatalf(
				"%s: #%d-th point values are not equal: %d (%s) vs %d (%s)",
				spec, i,
				p0.value, p0.bucket.item,
				p1.value, p1.bucket.item,
			)
		}
		i0 := itemString(p0.bucket.item)
		i1 := itemString(p1.bucket.item)
		if i0 != i1 {
			t.Fatalf(
				"%s: #%d-th point buckets are not equal: %s vs %s",
				spec, i, i0, i1,
			)
		}
	}
}

func itemString(x Item) string {
	var sb strings.Builder
	_, err := x.WriteTo(&sb)
	if err != nil {
		panic(fmt.Sprintf("item WriteTo() error: %v", err))
	}
	return sb.String()
}

type StringItem string

func (s StringItem) WriteTo(w io.Writer) (int64, error) {
	n, err := io.WriteString(w, string(s))
	return int64(n), err
}

type IntItem int

func (n IntItem) WriteTo(w io.Writer) (int64, error) {
	m, err := w.Write(intSuffix(int(n)))
	return int64(m), err
}
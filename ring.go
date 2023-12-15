package hashring

import (
	"container/list"
	"fmt"
	"hash"
	"io"
	"math"
	"sync"

	"github.com/cespare/xxhash/v2"
	"github.com/gobwas/avl"
)

const DefaultMagicFactor = 1020

type Item interface {
	io.WriterTo
}

// Ring is a consistent hashing hashring.
// It is goroutine safe. Ring instances must not be copied.
// The zero value for Ring is an empty ring ready to use.
type Ring struct {
	// Hash is an optional function used to build up a new 64-bit hash function
	// for further hash values calculation.
	Hash func() hash.Hash64

	// MagicFactor is an optional number of "virtual" points on the ring per
	// item. The higher this number, the more equal distribution of objects
	// this ring produces and the more time is needed to update the ring.
	//
	// MagicFactor is the maximum number of points which can be placed on ring
	// for a single item. That is, item having max weight will have this amount
	// of points.
	//
	// If MagicFactor is zero, then the DefaultMagicFactor is used. For most
	// applications the default value is fine enough.
	MagicFactor int

	// hashPool is a pool of reusable hash functions.
	hashPool sync.Pool

	// mu serializes write-only opearations on the ring.
	// It should be held when doing insert/update/delete operations, which in
	// turn lead to ring rebuild.
	mu sync.Mutex

	// buckets is a mapping of a non-suffixed digest of an item to a bucket.
	// It is protected by r.mu mutex.
	buckets map[uint64]*bucket

	// collisions is a mapping of collided point value to a tree of all points
	// having same value in their generations.
	// It is protected by r.mu mutex.
	collisions map[uint64]avl.Tree // tree<collision>

	// fix is a list of points required to be fixed.
	// It's filled only during ring mutation and drained in the end of it.
	// It is protected by r.mu mutex.
	fix list.List // list<*point>

	// minWeight holds minimum weight of item on the ring.
	// It is protected by r.mu mutex.
	minWeight float64
	// maxWeight holds maximum weight of item on the ring.
	// It is protected by r.mu mutex.
	maxWeight float64

	// ringMu serializes read & write operations on the tree holding bucket
	// points.
	// It's read-end should be held when reading the tree data.
	// It's write-end should be held when tree pointer is being updated.
	ringMu sync.RWMutex

	// ring is a tree holding bucket points.
	// It's protected by r.mu and r.ringMu mutex.
	// Note that r.mu mutex should be held while preparing new (mutated)
	// version of the tree.
	ring avl.Tree // tree<*point>

	trace traceRing
}

// Insert puts item x with weight w onto the ring.
// It returns non-nil error when x already exists on the ring.
// If weight is less or equal to zero Insert() panics.
func (r *Ring) Insert(x Item, w float64) error {
	if w <= 0 {
		panic("hashring: weight must be greater than zero")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	id := r.digest(x)
	_, has := r.buckets[id]
	if has {
		return fmt.Errorf("hashring: item already exists")
	}

	if r.buckets == nil {
		r.buckets = make(map[uint64]*bucket)
	}
	r.buckets[id] = newBucket(id, x, w)
	r.updateWeight(w)
	r.rebuild()

	return nil
}

// Update updates item's x weight on the ring.
// It returns non-nil error when x doesn't exist on the ring.
// If weight is less or equal to zero Update() panics.
func (r *Ring) Update(x Item, w float64) error {
	if w <= 0 {
		panic("hashring: weight must be greater than zero")
	}
	return r.update(x, w)
}

// Delete removes item x from the ring.
// It returns non-nil error when x doesn't exist on the ring.
func (r *Ring) Delete(x Item) error {
	return r.update(x, 0)
}

// Get returns mapping of v to previously inserted item.
// Returned item is nil only when ring is empty.
func (r *Ring) Get(v Item) Item {
	d := r.digest(v)

	r.ringMu.RLock()
	item := r.ring.Successor(search(d))
	if item == nil {
		item = r.ring.Min()
	}
	r.ringMu.RUnlock()

	if item == nil {
		return nil
	}
	return item.(*point).bucket.item
}

func (r *Ring) Has(x Item) bool {
	d := r.digest(x)

	r.ringMu.RLock()
	defer r.ringMu.RUnlock()

	_, has := r.buckets[d]
	return has
}

func (r *Ring) update(x Item, w float64) error {
	id := r.digest(x)

	r.mu.Lock()
	defer r.mu.Unlock()

	b, has := r.buckets[id]
	if !has {
		return fmt.Errorf("hashring: item doesn't exist")
	}

	prev := b.weight
	b.weight = w

	r.changeWeight(prev, w)
	r.rebuild()

	return nil
}

// r.mu must be held.
func (r *Ring) changeWeight(prev, next float64) {
	if prev != r.minWeight && prev != r.maxWeight {
		r.updateWeight(next)
		return
	}
	r.minWeight = 0
	r.maxWeight = 0
	for _, b := range r.buckets {
		if b.weight > 0 {
			r.updateWeight(b.weight)
		}
	}
}

// r.mu must be held.
func (r *Ring) updateWeight(w float64) {
	if r.minWeight == 0 || w < r.minWeight {
		r.minWeight = w
	}
	if r.maxWeight == 0 || w > r.maxWeight {
		r.maxWeight = w
	}
}

func (r *Ring) digest(src io.WriterTo, suffix ...byte) uint64 {
	h, _ := r.hashPool.Get().(hash.Hash64)
	if h == nil {
		if r.Hash != nil {
			h = r.Hash()
		} else {
			h = xxhash.New()
		}
	}
	defer func() {
		h.Reset()
		r.hashPool.Put(h)
	}()

	_, err := src.WriteTo(h)
	if err == nil {
		_, err = h.Write(suffix)
	}
	if err != nil {
		panic(fmt.Sprintf("hashring: digest error: %v", err))
	}
	return h.Sum64()
}

// r.mu must be held.
func (r *Ring) insertPoint(tree avl.Tree, p *point) (_ avl.Tree, inserted bool) {
	trace := r.trace.onInsert(p)
	defer func() {
		trace.onDone(inserted)
	}()

	if c := r.collisions[p.value()]; c.Size() != 0 {
		r.trace.onFixNeeded(p)
		r.collisions[p.value()] = mustInsertTree(c, collision{p})
		r.fix.PushBack(p)
		return tree, false
	}
	tree, existing := tree.Insert(p)
	if existing == nil {
		return tree, true
	}
	d := existing.(*point)
	trace.onCollision(d)
	// Collision detected.
	tree, existed := tree.Delete(d)
	if existed == nil {
		panic("hashring: internal error")
	}

	if r.collisions == nil {
		r.collisions = make(map[uint64]avl.Tree)
	}
	c := r.collisions[p.value()]
	c = mustInsertTree(c, collision{p})
	c = mustInsertTree(c, collision{d})
	r.collisions[p.value()] = c

	assertNotExists(tree, d)
	assertNotExists(tree, p)
	r.fix.PushBack(d)
	r.fix.PushBack(p)
	r.trace.onFixNeeded(d)
	r.trace.onFixNeeded(p)

	return tree, false
}

// r.mu must be held.
func (r *Ring) deletePoint(tree avl.Tree, p *point) (_ avl.Tree, removed bool) {
	trace := r.trace.onDelete(p)
	defer func() {
		trace.onDone(removed)
	}()

	var item avl.Item
	tree, item = tree.Delete(p)
	if item == nil {
		return tree, false
	}
	var (
		toDelete list.List
		toInsert list.List
	)
	for {
		done := trace.onProcessing(p)
		for p.generation() > 0 {
			// Rollback one generation back.
			p.rewind()

			c, has := r.collisions[p.value()]
			if !has {
				// We are processing twin here, and collisions were removed
				// already.
				continue
			}
			c = mustDeleteTree(c, collision{p})
			if c.Size() > 1 {
				// There are more than one twins remaining, so don't cleanup
				// them yet.
				r.collisions[p.value()] = c
				continue
			}
			delete(r.collisions, p.value())

			twin := c.Min().(collision).point
			trace.onTwinDelete(twin)
			// Delete twin from the ring, but defer its cleanup.
			var existed avl.Item
			tree, existed = tree.Delete(twin)
			if existed != nil {
				// We have to first cleanup all collisions of current point, so
				// enqueue twins in the queue to delete later.
				toDelete.PushBack(twin)
				toInsert.PushBack(twin)
			}
		}
		done()
		if toDelete.Len() == 0 {
			break
		}
		p = toDelete.Remove(toDelete.Front()).(*point)
	}
	// Insert back twins removed above (they can collide as well).
	for el := toInsert.Front(); el != nil; el = toInsert.Front() {
		p := toInsert.Remove(el).(*point)
		trace.onTwinRestore(p)
		tree, _ = r.insertPoint(tree, p)
	}

	return tree, true
}

func (r *Ring) magicFactor() float64 {
	if m := r.MagicFactor; m > 0 {
		return float64(m)
	}
	return DefaultMagicFactor
}

// r.mu must be held.
func (r *Ring) numPoints() func(float64) int {
	if r.maxWeight == 0 {
		return func(float64) int { return 0 }
	}
	return line(
		r.maxWeight, r.magicFactor(),
		r.minWeight, math.Ceil(r.magicFactor())*(r.minWeight/r.maxWeight),
	)
}

// r.mu must be held.
func (r *Ring) rebuild() {
	numPoints := r.numPoints()

	r.ringMu.RLock()
	root := r.ring
	r.ringMu.RUnlock()

	for {
		for id, b := range r.buckets {
			var size int
			if b.weight != 0 {
				size = numPoints(b.weight)
			}
			for i := len(b.points); i > size; i-- {
				p := b.points[i-1]
				b.points = b.points[:i-1]
				root, _ = r.deletePoint(root, p)
			}
			for i := len(b.points); i < size; i++ {
				v := r.digest(b.item, encodeSuffix(0, i)...)
				p := newPoint(b, i, v)
				b.points = append(b.points, p)
				root, _ = r.insertPoint(root, p)
			}
			if b.weight == 0 {
				delete(r.buckets, id)
			}
		}
		for el := r.fix.Front(); el != nil; el = r.fix.Front() {
			p := r.fix.Remove(el).(*point)

			trace := r.trace.onFix(p)
			assertNotExists(root, p)

			g := p.generation()
			v := r.digest(p.bucket.item, encodeSuffix(g+1, p.index)...)
			p.proceed(v)
			root, _ = r.insertPoint(root, p)

			trace.onDone()
		}
		if r.fix.Len() == 0 {
			break
		}
	}

	r.ringMu.Lock()
	r.ring = root
	r.ringMu.Unlock()
}

func line(x0, y0, x1, y1 float64) func(float64) int {
	if x0 == x1 && y0 != y1 {
		panic(fmt.Sprintf(
			"hashring: internal error: malformed points: [<%.2f, %.2f>, <%.2f, %.2f>]",
			x0, y0, x1, y1,
		))
	}
	if x0 == x1 {
		return func(x float64) int {
			return int(y0 + 0.5)
		}
	}
	m := (y1 - y0) / (x1 - x0) // Slope of a line.
	return func(x float64) int {
		n := m*(x-x0) + y0
		return int(n + 0.5)
	}
}

func mustInsertTree(tree avl.Tree, x avl.Item) avl.Tree {
	tree, existing := tree.Insert(x)
	if existing != nil {
		panic("hashring: internal error: mustInsert failed")
	}
	return tree
}

func mustDeleteTree(tree avl.Tree, x avl.Item) avl.Tree {
	tree, existed := tree.Delete(x)
	if existed == nil {
		panic("hashring: internal error: mustDelete failed")
	}
	return tree
}

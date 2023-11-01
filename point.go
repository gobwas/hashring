package hashring

import "github.com/gobwas/avl"

// point represents a point on the ring.
// To handle collisions properly it may change its value to another one,
// increasing its generation by one.
type point struct {
	// bucket is a bucket where point belongs to.
	bucket *bucket

	// index is a constant index of the point within bucket.
	index int

	// val is a current value of the point.
	// It might be changed if point collides with another one.
	val uint64

	// stack holds a history of point values.
	// It's non-nil only if point collides with another one.
	stack []uint64
}

func newPoint(b *bucket, i int, v uint64) *point {
	return &point{
		bucket: b,
		index:  i,
		val:    v,
	}
}

func (p *point) generation() int {
	return len(p.stack)
}

func (p *point) proceed(v uint64) {
	p.stack = append(p.stack, p.val)
	p.val = v
}

func (p *point) rewind() {
	n := len(p.stack)
	p.val = p.stack[n-1]
	p.stack = p.stack[:n-1]
}

func (p *point) value() uint64 {
	return p.val
}

func (p *point) Compare(x avl.Item) int {
	return compare(p.val, x.(*point).val)
}

type collision struct {
	*point
}

func (c collision) Compare(x avl.Item) int {
	p0 := c.point
	p1 := x.(collision).point
	if x := compare(p0.bucket.id, p1.bucket.id); x != 0 {
		return x
	}
	return p0.index - p1.index
}

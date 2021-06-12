package hashring

import (
	"github.com/gobwas/avl"
)

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

type point struct {
	bucket *bucket
	index  int

	// These fields are dynamic.
	value uint64
	stack []uint64
}

func (p *point) generation() int {
	return len(p.stack)
}

func newPoint(b *bucket, i int, v uint64) *point {
	return &point{
		bucket: b,
		index:  i,
		value:  v,
	}
}

func (p *point) pushValue(v uint64) {
	p.stack = append(p.stack, p.value)
	p.value = v
}

func (p *point) backward() uint64 {
	n := len(p.stack)
	p.value = p.stack[n-1]
	p.stack = p.stack[:n-1]
	return p.value
}

func (p *point) Compare(x avl.Item) int {
	return compare(p.value, x.(*point).value)
}

type bucket struct {
	id     uint64
	points []*point
	item   Item
	weight float64
}

func newBucket(id uint64, item Item, weight float64) *bucket {
	return &bucket{
		id:     id,
		item:   item,
		weight: weight,
	}
}

type search uint64

func (s search) Compare(x avl.Item) int {
	return compare(uint64(s), x.(*point).value)
}

func compare(x0, x1 uint64) int {
	if x0 < x1 {
		return -1
	}
	if x0 > x1 {
		return 1
	}
	return 0
}

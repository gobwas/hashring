package hashring

import (
	"github.com/gobwas/avl"
)

func compare(x0, x1 uint64) int {
	if x0 < x1 {
		return -1
	}
	if x0 > x1 {
		return 1
	}
	return 0
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
	return compare(uint64(s), x.(*point).val)
}

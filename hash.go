package hashring

import (
	"encoding/binary"
)

const (
	intSize = 4 << (^uint(0) >> 63)
)

func encodeSuffix(xs ...int) []byte {
	p := make([]byte, intSize*len(xs))
	for i, x := range xs {
		dst := p[i*intSize:]
		switch intSize {
		case 4:
			binary.LittleEndian.PutUint32(dst, uint32(x))
		case 8:
			binary.LittleEndian.PutUint64(dst, uint64(x))
		}
	}
	return p
}

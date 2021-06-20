package hashring

import (
	"encoding/binary"
)

const (
	intSize = 4 << (^uint(0) >> 63)
)

var magic = (func() []byte {
	p := make([]byte, intSize)
	for i := range p {
		p[i] = byte(i)
	}
	return p
})()

func encodeSuffix(xs ...int) []byte {
	p := make([]byte, intSize*(len(xs)+1)) // is for zeroed value.
	copy(p, magic)
	for i, x := range xs {
		dst := p[(i+1)*intSize:]
		switch intSize {
		case 4:
			binary.LittleEndian.PutUint32(dst, uint32(x))
		case 8:
			binary.LittleEndian.PutUint64(dst, uint64(x))
		}
	}
	return p
}

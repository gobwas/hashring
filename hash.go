package hashring

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync"

	"github.com/cespare/xxhash/v2"
)

var digestTestHook func(io.WriterTo, ...byte) uint64

const (
	intSize = 4 << (^uint(0) >> 63)
	maxInt  = int(^uint(0) >> 1) // math.MaxInt since Go 1.17.
)

func suffixInts(bts []byte) []int {
	ret := make([]int, len(bts)/intSize)
	for i, j := intSize, 0; i <= len(bts); i, j = i+intSize, j+1 {
		src := bts[i-intSize : i]
		switch intSize {
		case 4:
			ret[j] = int(binary.LittleEndian.Uint32(src))
		case 8:
			ret[j] = int(binary.LittleEndian.Uint64(src))
		}
	}
	return ret
}

func intSuffix(xs ...int) []byte {
	p := make([]byte, intSize*len(xs))
	for i, x := range xs {
		switch intSize {
		case 4:
			binary.LittleEndian.PutUint32(p[i*4:], uint32(x))
		case 8:
			binary.LittleEndian.PutUint64(p[i*8:], uint64(x))
		}
	}
	return p
}

func digest(src io.WriterTo, suffix ...byte) (sum uint64) {
	if !debug || digestTestHook == nil {
		return xxDigest(src, suffix...)
	}
	return digestTestHook(src, suffix...)
}

var xxPool = sync.Pool{
	New: func() interface{} {
		return xxhash.New()
	},
}

func xxDigest(src io.WriterTo, suffix ...byte) (sum uint64) {
	d := xxPool.Get().(*xxhash.Digest)
	defer func() {
		d.Reset()
		xxPool.Put(d)
	}()
	_, err := src.WriteTo(d)
	if err == nil {
		_, err = d.Write(suffix)
	}
	if err != nil {
		panic(fmt.Sprintf("hashring: item hash has failed: %v", err))
	}
	return d.Sum64()
}

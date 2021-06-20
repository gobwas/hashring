package hashring

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash"
	"strconv"
	"strings"
	"testing"

	"github.com/cespare/xxhash/v2"
)

type digestArgs struct {
	item   string
	n      int
	suffix [2]int
}

func (d digestArgs) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%#q", d.item)
	sb.WriteByte('[')
	for i := 0; i < d.n; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(strconv.Itoa(d.suffix[i]))
	}
	sb.WriteByte(']')
	return sb.String()
}

func digestCall(s string, suffix ...int) digestArgs {
	args := digestArgs{item: s}
	if len(suffix) > 2 {
		panic(fmt.Sprintf(
			"digest hook: too many suffix ints for %#q: %v",
			s, suffix,
		))
	}
	args.n = copy(args.suffix[:], suffix)
	return args
}

func setupDigest(t *testing.T, r *Ring, values map[digestArgs]uint64) {
	r.Hash = func() hash.Hash64 {
		return &hash64{
			t:      t,
			values: values,
		}
	}
}

type hash64 struct {
	t      testing.TB
	values map[digestArgs]uint64
	buf    bytes.Buffer
}

func (h *hash64) Write(p []byte) (int, error) {
	return h.buf.Write(p)
}

func (h *hash64) Sum(b []byte) []byte {
	panic("hashring: hash Sum() must not be called")
}

func (h *hash64) Reset() {
	h.buf.Reset()
}

func (h *hash64) Size() int {
	return 8
}

func (h *hash64) BlockSize() int {
	return 1
}

func (h *hash64) Sum64() uint64 {
	item, suff := splitSuffix(h.buf.Bytes())
	call := digestCall(string(item), suff...)
	v, has := h.values[call]
	if has {
		h.t.Logf("using digest value for call %s: %d", call, v)
		return v
	}
	return xxDigest(h.buf.Bytes())
}

func xxDigest(p []byte) uint64 {
	h := xxhash.New()
	_, err := h.Write(p)
	if err != nil {
		panic(err)
	}
	return h.Sum64()
}

func splitSuffix(bts []byte) (_ []byte, xs []int) {
	i := bytes.Index(bts, magic)
	if i == -1 {
		return bts, nil
	}

	ret := bts[:i]
	suf := bts[i+intSize:]

	xs = make([]int, len(suf)/intSize)
	if len(xs) != 2 {
		panic(fmt.Sprintf(
			"unexpected size of hash suffix: %d (%#q)",
			len(xs), bts,
		))
	}
	for i, j := intSize, 0; i <= len(suf); i, j = i+intSize, j+1 {
		src := suf[i-intSize : i]
		switch intSize {
		case 4:
			xs[j] = int(binary.LittleEndian.Uint32(src))
		case 8:
			xs[j] = int(binary.LittleEndian.Uint64(src))
		}
	}

	return ret, xs
}

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

// splitSuffix splits given bytes slice by the item bytes and its suffix,
// produced by a ring.
//
// NOTE: it's assumed that item bytes are no longer than 2*intSize (intSize is
// 4 or 8 bytes depending on arch).
func splitSuffix(bts []byte) (item []byte, ints []int) {
	n := len(bts)
	s := 2 * intSize
	if n <= s {
		// No suffix added.
		return bts, nil
	}
	suf := bts[n-s:]
	return bts[:s], []int{
		decodeInt(suf[0*intSize:]),
		decodeInt(suf[1*intSize:]),
	}
}

func decodeInt(src []byte) int {
	switch intSize {
	case 4:
		return int(binary.LittleEndian.Uint32(src))
	case 8:
		return int(binary.LittleEndian.Uint64(src))
	default:
		panic("unexpected int size")
	}
}

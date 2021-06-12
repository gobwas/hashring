package hashring

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
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

func setupDigest(t *testing.T, values map[digestArgs]uint64) {
	undo := setupDigestHook(func(w io.WriterTo, suffix ...byte) uint64 {
		var sb strings.Builder
		_, err := w.WriteTo(&sb)
		if err != nil {
			t.Fatal(err)
		}
		if len(suffix) > intSize*2 {
			t.Fatalf(
				"digest hook: too many suffix bytes for %#q: %v",
				sb.String(), suffix,
			)
		}
		call := digestCall(sb.String(), suffixInts(suffix)...)
		v, has := values[call]
		if has {
			t.Logf("using digest value for call %s: %d", call, v)
			return v
		}
		return xxDigest(w, suffix...)
	})
	t.Cleanup(undo)
}

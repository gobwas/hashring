// +build !debug

package hashring

import (
	"io"

	"github.com/gobwas/avl"
)

const debug = false

func setupDigestHook(fn func(io.WriterTo, ...byte) uint64) func() {
	panic("setupDigestHook() can only be called with `debug` buildtag")
}

func assertNotExists(avl.Tree, *point) {}
func setupRingTrace(r *Ring)           {}

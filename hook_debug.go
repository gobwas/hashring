// +build hashring_debug

package hashring

import (
	"fmt"
	"log"
	"strings"

	"github.com/gobwas/avl"
)

const debug = true

func assertNotExists(tree avl.Tree, p *point) {
	if x := tree.Search(p); x != nil && x.(*point) == p {
		// NOTE: x could be another point collided with p.
		panic(fmt.Sprintf(
			"hashring: internal error: point must not exist on the ring",
		))
	}
}

func setupRingTrace(r *Ring) {
	log.SetFlags(0)

	var depth int
	enter := func() {
		depth++
		log.SetPrefix(strings.Repeat(" ", depth*4))
	}
	leave := func() {
		depth--
		log.SetPrefix(strings.Repeat(" ", depth*4))
	}
	r.trace = r.trace.Compose(traceRing{
		OnInsert: func(p *point) traceRingInsert {
			log.Println("inserting:", pointInfo(p))
			enter()
			return traceRingInsert{
				OnDone: func(inserted bool) {
					leave()
					if inserted {
						log.Println("inserted")
					} else {
						log.Println("not inserted")
					}
				},
				OnCollision: func(prev *point) {
					log.Println("collision:")
					enter()
					log.Println("prev:", pointInfo(prev))
					log.Println("next:", pointInfo(p))
					leave()
				},
			}
		},
		OnDelete: func(p *point) traceRingDelete {
			log.Println("deleting:", pointInfo(p))
			enter()
			return traceRingDelete{
				OnDone: func(deleted bool) {
					leave()
					if deleted {
						log.Println("deleted")
					} else {
						log.Println("not deleted")
					}
				},
				OnProcessing: func(p *point) func() {
					log.Println("processing:", pointInfo(p))
					enter()
					return func() {
						leave()
						log.Println("processed")
					}
				},
				OnTwinDelete: func(p *point) {
					log.Println("deleting twin", pointInfo(p))
				},
				OnTwinRestore: func(p *point) {
					log.Println("restoring twin", pointInfo(p))
				},
			}
		},
		OnFixNeeded: func(p *point) {
			log.Println("enqueued for fix:", pointInfo(p))
		},
		OnFix: func(p *point) traceRingFix {
			log.Println("fixing:", pointInfo(p))
			enter()
			return traceRingFix{
				OnDone: func() {
					leave()
					fmt.Println("fixed")
				},
			}
		},
	})
}

func pointInfo(p *point) string {
	return fmt.Sprintf(
		"%p: %s[%d] %v %d",
		p, p.bucket.item, p.index, p.stack, p.val,
	)
}

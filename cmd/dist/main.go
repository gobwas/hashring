package main

import (
	"crypto/md5"
	"encoding/binary"
	"flag"
	"fmt"
	"hash"
	"io"
	"log"
	"math"
	"math/rand"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/gobwas/avl"
	"github.com/gobwas/hashring"
)

func main() {
	var (
		p        int    // Number of goroutines.
		n        int    // Number of objects.
		s        int    // Number of servers on the ring.
		lo       int    // Min magic factor.
		hi       int    // Max magic factor.
		fs       string // Comma-separated factors list.
		csv      bool
		hashFunc string // Optional hash function name.

		verbose bool
		silent  bool
	)
	flag.IntVar(&p,
		"parallelism", runtime.NumCPU(),
		"number of concurrent processors",
	)
	flag.IntVar(&n,
		"objects", 1e6,
		"number of objects to spread on ring",
	)
	flag.IntVar(&s,
		"servers", 10,
		"number of servers to place on ring",
	)
	flag.IntVar(&lo,
		"lo", 0,
		"magic factor to start from",
	)
	flag.IntVar(&hi,
		"hi", 0,
		"magic factor to end at",
	)
	flag.StringVar(&fs,
		"factors", "",
		"comma-separated list of magic factors",
	)
	flag.StringVar(&hashFunc,
		"hash", "",
		"custom hash function to be used",
	)
	flag.BoolVar(&verbose,
		"v", false,
		"be verbose",
	)
	flag.BoolVar(&silent,
		"s", false,
		"be silent",
	)
	flag.BoolVar(&csv,
		"csv", true,
		"print csv to standard output",
	)

	flag.Parse()

	logf := func(f string, args ...interface{}) {
		if !verbose {
			return
		}
		log.Printf(f, args...)
	}
	printf := func(f string, args ...interface{}) {
		if silent {
			return
		}
		fmt.Fprintf(os.Stderr, f, args...)
	}

	// Prepare servers to be put on ring(s).
	servers := make([]StringItem, s)
	seenSrv := make(map[string]bool)
	for i := 0; i < s; {
		var b [4]byte
		_, err := rand.Read(b[:])
		if err != nil {
			panic(err)
		}
		ip := net.IPv4(b[0], b[1], b[2], b[3])
		s := ip.String()
		if seenSrv[s] {
			logf("#%d server duplicated; repeat", i)
			continue
		}
		seenSrv[s] = true
		servers[i] = StringItem(s)
		i++
	}
	logf("%d servers are ready", len(servers))

	// Prepare objects to be spread across servers on ring(s).
	objects := make([]hashring.Item, n)
	seenObj := make(map[string]bool)
	for i := 0; i < n; {
		s := fmt.Sprintf("%016x", rand.Intn(math.MaxInt64))
		if seenObj[s] {
			logf("#%d object duplicated; repeat", i)
			continue
		}
		seenObj[s] = true
		objects[i] = StringItem(s)
		i++
	}
	logf("%d objects are ready", len(objects))

	// Prepare list of magic factors. We merge here factors range (from `lo` to
	// `hi`) with manually specified factors in `fs`.
	// We use tree to autofix duplicates (if any).
	var factors avl.Tree
	for _, s := range strings.Split(fs, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		f, err := strconv.Atoi(s)
		if err != nil {
			panic(err)
		}
		factors, _ = factors.Insert(factor(f))
	}
	for f := lo; f < hi; f++ {
		factors, _ = factors.Insert(factor(f))
	}
	logf("%d factors are ready", factors.Size())

	mean := float64(n) / float64(s)

	var (
		work    = make(chan int)
		stop    = make(chan struct{})
		done    = make(chan struct{}, p)
		results = make(chan result, 1)
	)
	for i := 0; i < p; i++ {
		go func() {
			defer func() {
				done <- struct{}{}
			}()
			distribution := make(map[string]int, len(servers))
			for {
				var f int
				select {
				case <-stop:
					return
				case f = <-work:
					// Process below.
				}

				r := hashring.Ring{
					MagicFactor: f,
				}
				switch hashFunc {
				case "":
				case "md5":
					r.Hash = func() hash.Hash64 {
						return newHash64(md5.New())
					}
				default:
					panic(fmt.Sprintf("unexpected hash function: %q", hashFunc))
				}

				start := time.Now()
				for _, item := range servers {
					err := r.Insert(item, 1)
					if err != nil {
						panic(err)
					}
				}
				latency := time.Since(start)

				for _, obj := range objects {
					item := r.Get(obj)
					if item == nil {
						panic(fmt.Sprintf("empty item"))
					}
					s := string(item.(StringItem))
					distribution[s]++
				}
				var variance float64
				for key, d := range distribution {
					variance += math.Pow(float64(d)-mean, 2)
					distribution[key] = 0
				}
				// Divide by number of servers as for mean.
				variance /= float64(s)
				results <- result{
					f:       f,
					latency: latency,
					stddev:  math.Sqrt(variance),
				}
			}
		}()
	}

	go func() {
		factors.InOrder(func(x avl.Item) bool {
			select {
			case <-stop:
				return false
			case work <- int(x.(factor)):
				return true
			}
		})
		close(stop)
		for i := 0; i < p; i++ {
			<-done
		}
		close(results)
	}()

	var t avl.Tree
	for r := range results {
		t, _ = t.Insert(r)
		printf(".")
		if n := t.Size(); n%80 == 0 {
			f := factors.Size()
			printf(
				"%d/%d(%.1f%%)\n",
				n, f,
				float64(n)/float64(f)*100, // Progress percentage.
			)
		}
	}
	printf("\n")

	tw := tabwriter.NewWriter(os.Stdout, 2, 2, 2, ' ', 0)
	t.InOrder(func(x avl.Item) bool {
		r := x.(result)
		var (
			devPct  = r.stddev / float64(n) * 100
			diffPct = float64(r.maxDiff) / float64(n) * 100
		)
		logf(
			"%04d: stddev=%.2f(%.2f%%) maxdiff=%d(%.2f%%) latency=%s\n",
			r.f,
			r.stddev, devPct,
			r.maxDiff, diffPct,
			r.latency,
		)
		if csv {
			fmt.Fprintf(tw,
				"%d,\t%.4f,\t%.4f,\t%.2f\n",
				r.f, devPct, diffPct,
				r.latency.Seconds()*1000,
			)
		}
		return true
	})
	tw.Flush()

	printf("OK")
}

type result struct {
	f       int
	latency time.Duration
	stddev  float64
	maxDiff int
}

func (r result) Compare(x avl.Item) int {
	return r.f - x.(result).f
}

type StringItem string

func (s StringItem) WriteTo(w io.Writer) (int64, error) {
	n, err := io.WriteString(w, string(s))
	return int64(n), err
}

type factor int

func (f factor) Compare(x avl.Item) int {
	return int(f - x.(factor))
}

type hash64 struct {
	hash.Hash
}

func newHash64(h hash.Hash) hash.Hash64 {
	return &hash64{Hash: h}
}

func (h *hash64) Sum64() uint64 {
	if h.Size() < 8 {
		panic("too small hash")
	}
	sum := h.Sum(nil)
	return binary.LittleEndian.Uint64(sum)
}

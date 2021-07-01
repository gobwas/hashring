# hashring

[![GoDoc][godoc-image]][godoc-url]
[![CI][ci-badge]][ci-url]

> Consistent hashing hashring implementation.

# Overview

This is an implementation of the consistent hashing hashring data structure.

In general, consistent hashing is all about mapping objects from a very big set
of values (e.g., request id) to objects from a quite small set (e.g., server
address). The word "consistent" means that it can produce consistent mapping on
different machines or processes without additional state exchange and
communication.

For more theory about the subject please see this [great
document][stanford-doc].

For more info about the package please read the [docs][godoc-url].

# The Why?

This is an approach for load distribution across servers which I found
convinient to be used in a few projects I had worked on in the past. 

I think it is good to implement it from scratch to synthesize all my experience
working with it and share it with the community in hope it will help to build
something good.

The key points of the package:
1) Efficient concurrent read operations
2) Correct handling of write operations (order of insertions on different
processes doesn't matter; hash collisions are handled carefully).

# Installation

```bash
go get github.com/gobwas/hashring
```

# Usage

```go
package main

import (
	"strings"
	"io"

	"github.com/gobwas/hashring"
)

func main() {
	var ring hashring.Ring
	_ = ring.Insert(StringItem("server01"))
	_ = ring.Insert(StringItem("server02"))
	_ = ring.Insert(StringItem("server03"))
	_ = ring.Insert(StringItem("server04"))

	ring.Get(StringItem("user01")) // server04
	ring.Get(StringItem("user02")) // server04
	ring.Get(StringItem("user03")) // server02
	ring.Get(StringItem("user04")) // server01
}

type StringItem string

func (s StringItem) WriteTo(w io.Writer) (int64, error) {
	n, err := io.WriteString(w, string(s))
	return int64(n), err
}
```

# Contributing

If you find some bug or want to improve this package in any way feel free to
file an issue to discuss your findings or ideas.

## Debugging

Note that this package comes with built in _debug_ tracing (which only takes
place if you pass `hashring_trace` build tag, thanks to [gtrace][gtrace]
zero-cost tracing).

This means that you can make hashring to print out each meaningful step it does
to understand better what's happening under the hood.

It also consumes `hashring_debug` build tag, which allows you to hook into hash
calculation process and override the value it returns. For example, this was
useful to test hash collisions.

## Magic factor

Magic factor is a number of "virtual" nodes each item gets on a ring. The
higher this number, the more equal distribution of objects this ring produces
and the more time is needed to update the ring.

The default value is picked through testing different sets of servers and
objects, however, for some datasets it may be enough to have smaller (or
higher) value of magic factor. There is a branch called [magic][magic] which
contains code used to generate statistics on distribution of objects depending
on magic factor value.

Here is the sample result of distribution of 10M objects across 10 servers with
different magic factor values:
<img alt="Magic factor plot" src="https://github.com/gobwas/hashring/blob/magic/magicfactor.png" width="800">

[godoc-image]:  https://godoc.org/github.com/gobwas/hashring?status.svg
[godoc-url]:    https://godoc.org/github.com/gobwas/hashring
[ci-badge]:     https://github.com/gobwas/hashring/actions/workflows/main.yml/badge.svg?branch=main
[ci-url]:       https://github.com/gobwas/hashring/actions/workflows/main.yml
[stanford-doc]: https://theory.stanford.edu/~tim/s16/l/l1.pdf
[gtrace]:       https://github.com/gobwas/gtrace
[magic]:        https://github.com/gobwas/hashring/tree/magic
[magic-image]:  https://github.com/gobwas/hashring/blob/magic/magicfactor.png

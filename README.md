# hashring

[![GoDoc][godoc-image]][godoc-url]
[![CI][ci-badge]][ci-url]

> Consistent hashing hashring implementation.

# Overview

This is an implementation of the consistent hashing hashring data structure.
For more info please read the [docs][godoc-url].

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


[godoc-image]: https://godoc.org/github.com/gobwas/hashring?status.svg
[godoc-url]:   https://godoc.org/github.com/gobwas/hashring
[ci-badge]:    https://github.com/gobwas/hashring/actions/workflows/main.yml/badge.svg?branch=main
[ci-url]:      https://github.com/gobwas/hashring/actions/workflows/main.yml

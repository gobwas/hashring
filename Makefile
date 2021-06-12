sources:=$(filter-out %_test.go,$(wildcard *.go))

gtrace: go.mod go.sum
	go build -mod mod -o gtrace github.com/gobwas/gtrace/cmd/gtrace

hook_gtrace.go: $(sources) gtrace
	PATH=$(PWD):$(PATH) go generate ./...

.PHONY: test
test: gtrace hook_gtrace.go
	go test . -v -tags debug


.PHONY: clean
clean:
	rm -f hook_gtrace.go hook_gtrace_stub.go

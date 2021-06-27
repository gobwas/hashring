sources:=$(filter-out %_test.go,$(wildcard *.go))
sources:=$(filter-out %_gtrace.go,$(sources))

gtrace: go.mod go.sum
	go get github.com/gobwas/gtrace@v0.4.3
	go build -mod mod -o gtrace github.com/gobwas/gtrace/cmd/gtrace
	go mod tidy

hook_gtrace.go: $(sources) gtrace
	PATH=$(PWD):$(PATH) go generate ./...

.PHONY: generate
generate: hook_gtrace.go

.PHONY: test
test: gtrace hook_gtrace.go
	go test . -v -tags hashring_debug

.PHONY: clean
clean:
	rm -f hook_gtrace.go hook_gtrace_stub.go gtrace

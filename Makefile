.PHONY: build

GOFLAGS = -gcflags "all=-trimpath=$(PWD)" -asmflags "all=-trimpath=$(PWD)"

build:
	@go build -o bin/eco2exe $(GOFLAGS) ./cmd/eco2exe
	@go build -o bin/objdump $(GOFLAGS) ./cmd/objdump
	@go build -o bin/sioload $(GOFLAGS) ./cmd/sioload

gen:
	@go generate ./pkg/yaroze

GO_MODULE_NAME := $(shell awk '$$1=="module"{print $$2}' go.mod)

build:
	go build -ldflags "-s -w" -o ypd cmd/cli/main.go 

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o ypd-darwin-arm64 cmd/cli/main.go

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ypd-linux-amd64 cmd/cli/main.go

format:
	gofmt -w -s .
	goimports  -local $(GO_MODULE_NAME) -w .

clean:
	rm -rf ./ypd ./ypd-darwin-arm64 ./ypd-linux-amd64

.PHONY: format build build-darwin-arm64 build-linux-amd64 clean
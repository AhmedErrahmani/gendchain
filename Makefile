.PHONY: gendchain android ios gendchain-cross swarm evm all test clean
.PHONY: gendchain-linux gendchain-linux-386 gendchain-linux-amd64 gendchain-linux-mips64 gendchain-linux-mips64le
.PHONY: gendchain-linux-arm gendchain-linux-arm-5 gendchain-linux-arm-6 gendchain-linux-arm-7 gendchain-linux-arm64
.PHONY: gendchain-darwin gendchain-darwin-386 gendchain-darwin-amd64
.PHONY: gendchain-windows gendchain-windows-386 gendchain-windows-amd64
.PHONY: docker release

GOBIN = $(shell pwd)/build/bin
GO ?= latest

# Compare current go version to minimum required version. Exit with \
# error message if current version is older than required version.
# Set min_ver to the minimum required Go version such as "1.12"
min_ver := 1.12
ver = $(shell go version)
ver2 = $(word 3, ,$(ver))
cur_ver = $(subst go,,$(ver2))
ver_check := $(filter $(min_ver),$(firstword $(sort $(cur_ver) \
$(min_ver))))
ifeq ($(ver_check),)
$(error Running Go version $(cur_ver). Need $(min_ver) or higher. Please upgrade Go version)
endif

gendchain:
	cd cmd/gendchain; go build -o ../../bin/gendchain
	@echo "Done building."
	@echo "Run \"bin/gendchain\" to launch gendchain."

bootnode:
	cd cmd/bootnode; go build -o ../../bin/gendchain-bootnode
	@echo "Done building."
	@echo "Run \"bin/gendchain-bootnode\" to launch gendchain bootnode."

docker:
	docker build -t gendchain/gendchain .

all: bootnode gendchain

release:
	./release.sh

install: all
	cp bin/gendchain-bootnode $(GOPATH)/bin/gendchain-bootnode
	cp bin/gendchain $(GOPATH)/bin/gendchain

android:
	build/env.sh go run build/ci.go aar --local
	@echo "Done building."
	@echo "Import \"$(GOBIN)/gendchain.aar\" to use the library."

ios:
	build/env.sh go run build/ci.go xcode --local
	@echo "Done building."
	@echo "Import \"$(GOBIN)/gendchain.framework\" to use the library."

test:
	go test ./...

clean:
	rm -fr build/_workspace/pkg/ $(GOBIN)/*

# The devtools target installs tools required for 'go generate'.
# You need to put $GOBIN (or $GOPATH/bin) in your PATH to use 'go generate'.

devtools:
	env GOBIN= go get -u golang.org/x/tools/cmd/stringer
	env GOBIN= go get -u github.com/kevinburke/go-bindata/go-bindata
	env GOBIN= go get -u github.com/fjl/gencodec
	env GOBIN= go get -u github.com/golang/protobuf/protoc-gen-go
	env GOBIN= go install ./cmd/abigen
	@type "npm" 2> /dev/null || echo 'Please install node.js and npm'
	@type "solc" 2> /dev/null || echo 'Please install solc'
	@type "protoc" 2> /dev/null || echo 'Please install protoc'

# Cross Compilation Targets (xgo)

gendchain-cross: gendchain-linux gendchain-darwin gendchain-windows gendchain-android gendchain-ios
	@echo "Full cross compilation done:"
	@ls -ld $(GOBIN)/gendchain-*

gendchain-linux: gendchain-linux-386 gendchain-linux-amd64 gendchain-linux-arm gendchain-linux-mips64 gendchain-linux-mips64le
	@echo "Linux cross compilation done:"
	@ls -ld $(GOBIN)/gendchain-linux-*

gendchain-linux-386:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/386 -v ./cmd/gendchain
	@echo "Linux 386 cross compilation done:"
	@ls -ld $(GOBIN)/gendchain-linux-* | grep 386

gendchain-linux-amd64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/amd64 -v ./cmd/gendchain
	@echo "Linux amd64 cross compilation done:"
	@ls -ld $(GOBIN)/gendchain-linux-* | grep amd64

gendchain-linux-arm: gendchain-linux-arm-5 gendchain-linux-arm-6 gendchain-linux-arm-7 gendchain-linux-arm64
	@echo "Linux ARM cross compilation done:"
	@ls -ld $(GOBIN)/gendchain-linux-* | grep arm

gendchain-linux-arm-5:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/arm-5 -v ./cmd/gendchain
	@echo "Linux ARMv5 cross compilation done:"
	@ls -ld $(GOBIN)/gendchain-linux-* | grep arm-5

gendchain-linux-arm-6:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/arm-6 -v ./cmd/gendchain
	@echo "Linux ARMv6 cross compilation done:"
	@ls -ld $(GOBIN)/gendchain-linux-* | grep arm-6

gendchain-linux-arm-7:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/arm-7 -v ./cmd/gendchain
	@echo "Linux ARMv7 cross compilation done:"
	@ls -ld $(GOBIN)/gendchain-linux-* | grep arm-7

gendchain-linux-arm64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/arm64 -v ./cmd/gendchain
	@echo "Linux ARM64 cross compilation done:"
	@ls -ld $(GOBIN)/gendchain-linux-* | grep arm64

gendchain-linux-mips:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/mips --ldflags '-extldflags "-static"' -v ./cmd/gendchain
	@echo "Linux MIPS cross compilation done:"
	@ls -ld $(GOBIN)/gendchain-linux-* | grep mips

gendchain-linux-mipsle:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/mipsle --ldflags '-extldflags "-static"' -v ./cmd/gendchain
	@echo "Linux MIPSle cross compilation done:"
	@ls -ld $(GOBIN)/gendchain-linux-* | grep mipsle

gendchain-linux-mips64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/mips64 --ldflags '-extldflags "-static"' -v ./cmd/gendchain
	@echo "Linux MIPS64 cross compilation done:"
	@ls -ld $(GOBIN)/gendchain-linux-* | grep mips64

gendchain-linux-mips64le:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/mips64le --ldflags '-extldflags "-static"' -v ./cmd/gendchain
	@echo "Linux MIPS64le cross compilation done:"
	@ls -ld $(GOBIN)/gendchain-linux-* | grep mips64le

gendchain-darwin: gendchain-darwin-386 gendchain-darwin-amd64
	@echo "Darwin cross compilation done:"
	@ls -ld $(GOBIN)/gendchain-darwin-*

gendchain-darwin-386:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=darwin/386 -v ./cmd/gendchain
	@echo "Darwin 386 cross compilation done:"
	@ls -ld $(GOBIN)/gendchain-darwin-* | grep 386

gendchain-darwin-amd64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=darwin/amd64 -v ./cmd/gendchain
	@echo "Darwin amd64 cross compilation done:"
	@ls -ld $(GOBIN)/gendchain-darwin-* | grep amd64

gendchain-windows: gendchain-windows-386 gendchain-windows-amd64
	@echo "Windows cross compilation done:"
	@ls -ld $(GOBIN)/gendchain-windows-*

gendchain-windows-386:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=windows/386 -v ./cmd/gendchain
	@echo "Windows 386 cross compilation done:"
	@ls -ld $(GOBIN)/gendchain-windows-* | grep 386

gendchain-windows-amd64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=windows/amd64 -v ./cmd/gendchain
	@echo "Windows amd64 cross compilation done:"
	@ls -ld $(GOBIN)/gendchain-windows-* | grep amd64

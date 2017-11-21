MKDIR=mkdir -p
XARGS=xargs

OS=$(shell uname -s)

ROOT=$(PWD)

BINPATH=$(ROOT)/bin:$(ROOT)/contrib/bin
CONFPATH=$(ROOT)/conf

export GOPATH:=$(GOPATH):$(ROOT)
export PATH:=$(BINPATH):$(PATH)
GO_BIN_PATH=$(firstword $(subst :, ,$(GOPATH)))

.PHONY: build clean contrib deps fmt init install proto proto_dbg

VER=`git show --quiet --pretty=%H%d`
build: protocol fmt
	@#echo "package main\n var V = \"$(VER)\"" > src/server/v.go
	@cd src && go install -v *

clean:
	@git clean -dxf

log:
	@$(MKDIR) -p log

deps:
	@go get -u -v golang.org/x/tools/cmd/goimports
	@cd src && go get -t -v *

fmt:
	@cd src && goimports -w *

init: contrib/bin/protoc deps dirs

# Protocol Buffers Compiler
ifeq ($(OS), linux)
PROTOC_URL="https://github.com/google/protobuf/releases/download/v3.5.0/protoc-3.5.0-linux-x86_64.zip"
endif
ifeq ($(OS), Darwin)
PROTOC_URL="https://github.com/google/protobuf/releases/download/v3.5.0/protoc-3.5.0-osx-x86_64.zip"
endif
ifeq ($(OS), win32)
PROTOC_URL="https://github.com/google/protobuf/releases/download/v3.5.0/protoc-3.5.0-win32.zip"
endif

contrib/bin/protoc:
	@$(MKDIR) contrib
	@cd contrib && wget -O /tmp/protoc.zip $(PROTOC_URL)
	@cd contrib && unzip /tmp/protoc.zip

protoc: contrib/bin/protoc

test: protocol fmt
	@cd src && go test *

# protocol
PB_FILES := $(wildcard protocol/*.proto)

PROTOC_GEN_GO=$(GO_BIN_PATH)/bin/protoc-gen-go
PB_GO_DIR=src/protocol
PB_GO_FILES := $(patsubst protocol/%.proto,$(PB_GO_DIR)/%.pb.go,$(PB_FILES))
PKG_PROTO=pkg/$(OS)_amd64/protocol.a

proto_dbg:
	@echo $(GO_BIN_PATH)
	@echo $(PB_GO_DIR)
	@echo $(PB_GO_FILES)
	@echo $(PROTOC_GEN_TS)
	@echo $(PB_TS_DIR)
	@echo $(PB_TS_FILES)

protocol: $(PROTOC_GEN_GO) $(PB_GO_DIR) $(PKG_PROTO) $(PB_TS_DIR) $(PROTOC_GEN_TS) $(PB_TS_FILES)

$(PROTOC_GEN_GO):
	@go get -u -v github.com/golang/protobuf/protoc-gen-go

$(PB_GO_DIR)/:
	@mkdir -p $@

$(PB_GO_DIR)/%.pb.go: protocol/%.proto
	@echo $^

$(PKG_PROTO): $(PB_FILES) $(PB_GO_FILES)
	protoc -I=protocol/ --go_out=$(PB_GO_DIR) protocol/*.proto
	cd src && go install protocol

CURRENT_TIME:=$(shell date +%x_%H:%M:%S)

install:
	go fmt ./...
	goimports -l -w `find ./ -name '*.go'`
	go install -ldflags="-s -w -X github.com/CyrusJavan/tf-bench/cmd.version=built_from_source_$(CURRENT_TIME)"

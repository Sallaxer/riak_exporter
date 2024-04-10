VERSION := 0.0.4
FLAGS := CGO_ENABLED=0
build:
    @echo ">> building binaries" 
	$(FLAGS) go build -mod=mod -ldflags "-s -w -X main.version=$(VERSION)" -o riak_exporter

run: build 
	./riak_exporter

clean: 
	rm -f ./riak_exporter

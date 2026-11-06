.PHONY: test cover cover-html lint fuzz

COVER := cover.out

test:
	go test -race ./...

cover:
	go test -race -covermode=atomic -coverpkg=./... -coverprofile=$(COVER) ./...
	@go tool cover -func=$(COVER) | grep -v '100.0%' || true
	@go tool cover -func=$(COVER) | awk '/^total:/ { if ($$3 != "100.0%") { print "coverage " $$3 " < 100%"; exit 1 } else print "coverage 100%" }'

cover-html: cover
	go tool cover -html=$(COVER)

lint:
	golangci-lint run

fuzz:
	go test -run='^$$' -fuzz=FuzzBindJSON -fuzztime=30s .
	go test -run='^$$' -fuzz=FuzzBindQuery -fuzztime=30s .

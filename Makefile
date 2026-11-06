.PHONY: test cover cover-html lint fuzz

COVER := cover.out

# Modules with their own go.mod. go.work ties them together for local development.
SUBMODULES := playground

test:
	go test -race ./...
	@for m in $(SUBMODULES); do (cd $$m && go test -race ./...) || exit 1; done

cover:
	go test -race -covermode=atomic -coverpkg=./... -coverprofile=$(COVER) ./...
	@go tool cover -func=$(COVER) | grep -v '100.0%' || true
	@go tool cover -func=$(COVER) | awk '/^total:/ { if ($$3 != "100.0%") { print "coverage " $$3 " < 100%"; exit 1 } else print "coverage 100%" }'

	@for m in $(SUBMODULES); do (cd $$m && go test -race -covermode=atomic -coverprofile=$(COVER) ./... && \
		go tool cover -func=$(COVER) | awk -v m=$$m '/^total:/ { if ($$3 != "100.0%") { print m ": coverage " $$3 " < 100%"; exit 1 } else print m ": coverage 100%" }') || exit 1; done

cover-html: cover
	go tool cover -html=$(COVER)

lint:
	golangci-lint run ./... $(addsuffix /...,$(addprefix ./,$(SUBMODULES)))

fuzz:
	go test -run='^$$' -fuzz=FuzzBindJSON -fuzztime=30s .
	go test -run='^$$' -fuzz=FuzzBindQuery -fuzztime=30s .

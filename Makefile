SERVICE_DIR = nexusd

.PHONY: all vet test service clean install uninstall test-coverage coverage-badge coverage spec-conformance

all: vet test service

vet:
	go vet -all -composites=false ./...

test:
	go build ./examples/...
	go test -race ./...
	go test -race ./test -scheme=ws
	go test -race ./test -scheme=unix
	go test ./test -scheme=ws -serialize=msgpack
	go test ./test -scheme=tcp -serialize=msgpack
	go test ./test -scheme=ws -serialize=cbor -compress
	go test ./test -scheme=tcp -serialize=cbor
	go test ./test -scheme=wss
	go test ./test -scheme=tcps

benchmark:
	go test ./test -run=XXX -bench=.
	go test ./test -run=XXX -bench=. -scheme=ws
	go test ./test -run=XXX -bench=. -scheme=wss
	go test ./test -run=XXX -bench=. -scheme=tcp
	go test ./test -run=XXX -bench=. -scheme=tcps
	go test ./test -run=XXX -bench=. -scheme=ws -compress

# Run the full suite once with coverage instrumentation. The transport-matrix
# variants in `test` are not re-run here — they exercise transport edge cases,
# not different code paths in the router/wamp/client packages we measure.
test-coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out | tail -1 | awk '{print $$3}' > coverage.txt
	@printf 'total coverage: %s\n' "$$(cat coverage.txt)"

# Rewrite the README coverage badge line to a shields.io URL with the
# current percentage. shields.io renders the badge on the fly — no SVG file
# is committed.
coverage-badge: test-coverage
	go run ./scripts/coverage-badge

# Convenience: run the YAML wire-frame conformance harness only.
spec-conformance:
	go test ./spec/runner/...

# Top-level: tests with coverage + badge regen. Run before pushing.
coverage: coverage-badge

service: $(SERVICE_DIR)/nexusd

$(SERVICE_DIR)/nexusd:
	@$(MAKE) -C $(SERVICE_DIR)

install: service
	@$(MAKE) -C $(SERVICE_DIR) install

uninstall:
	@$(MAKE) -C $(SERVICE_DIR) uninstall

clean:
	@$(MAKE) -C $(SERVICE_DIR) clean
	@rm -f $(SERVICE_DIR)/*.log
	@GO111MODULE=off go clean ./...
	@GO111MODULE=off go clean -cache

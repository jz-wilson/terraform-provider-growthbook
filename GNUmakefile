default: fmt lint install generate

build:
	go build -v ./...

install: build
	go install -v ./...

lint:
	golangci-lint run

generate:
	cd tools; go generate ./...

fmt:
	gofmt -s -w -e .

test:
	go test -v -cover -timeout=120s -parallel=10 ./...

testacc:
	TF_ACC=1 go test -v -cover -timeout 120m ./...

# sweep deletes every tf-acc-* growthbook_feature and growthbook_sdk_connection
# object left in the GrowthBook instance pointed to by GROWTHBOOK_API_KEY /
# GROWTHBOOK_API_URL. It never touches objects without that prefix, and never
# touches projects or environments. Run this after a failed/aborted live
# acceptance run (TestAcc*_live tests, GROWTHBOOK_LIVE=1) to clean up objects
# that a mid-test failure left behind; see internal/provider/sweeper_test.go.
sweep:
	go test ./internal/provider -v -sweep=all -timeout 10m

.PHONY: fmt lint test testacc build install generate sweep

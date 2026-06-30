.PHONY: test test-e2e

test:
	go test ./...

test-e2e:
	go test -tags=e2e ./test/e2e -count=1 -v

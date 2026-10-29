test:
	go vet ./... && go test ./...

fmt:
	gofmt -l -w .

.PHONY: test fmt

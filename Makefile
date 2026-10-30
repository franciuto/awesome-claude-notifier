GOFLAGS_DIST = -trimpath -buildvcs=false -ldflags="-s -w"
GOBUILD = CGO_ENABLED=0 go build $(GOFLAGS_DIST)

test:
	go vet ./... && go test ./...

fmt:
	gofmt -l -w .

# The five Go binaries the dispatcher picks from; reproducible, so CI can diff bin/.
dist-go:
	mkdir -p bin/darwin bin/linux-amd64 bin/linux-arm64 bin/windows-amd64 bin/windows-arm64
	tmp=$$(mktemp -d) && \
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -o $$tmp/arm64 ./cmd/claude-notify && \
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $$tmp/amd64 ./cmd/claude-notify && \
	lipo -create $$tmp/arm64 $$tmp/amd64 -output bin/darwin/claude-notify; \
	rc=$$?; rm -rf $$tmp; exit $$rc
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o bin/linux-amd64/claude-notify ./cmd/claude-notify
	GOOS=linux GOARCH=arm64 $(GOBUILD) -o bin/linux-arm64/claude-notify ./cmd/claude-notify
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o bin/windows-amd64/claude-notify.exe ./cmd/claude-notify
	GOOS=windows GOARCH=arm64 $(GOBUILD) -o bin/windows-arm64/claude-notify.exe ./cmd/claude-notify

dist: dist-go
	./macos-helper/build.sh

.PHONY: test fmt dist-go dist

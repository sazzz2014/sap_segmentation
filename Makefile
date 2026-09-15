.PHONY: test vet build local-check
test:
	go test -race ./...
vet:
	go vet ./...
build:
	go build -o bin/sap_segmentationd ./cmd/sap_segmentationd
local-check:
	sh scripts/local-check.sh

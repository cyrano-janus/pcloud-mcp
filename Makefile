.PHONY: build test fuzz mutations check security clean
build:
	go build -trimpath -o bin/pcloud-remote ./cmd/pcloud-remote
	go build -trimpath -o bin/pcloud-mcp ./cmd/pcloud-mcp
	go build -trimpath -o bin/pcloud-auth ./cmd/pcloud-auth
	go build -trimpath -o bin/pcloud-upload ./cmd/pcloud-upload
test:
	go test -race -count=1 -timeout=90s ./...
	go vet ./...
fuzz:
	go test ./internal/config -run='^$$' -fuzz=FuzzConfig -fuzztime=30s -parallel=2
	go test ./internal/policy -run='^$$' -fuzz=FuzzName -fuzztime=30s -parallel=2
	go test ./internal/pcloud -run='^$$' -fuzz=FuzzDecode -fuzztime=30s -parallel=2
	go test ./internal/netguard -run='^$$' -fuzz=FuzzPublicIP -fuzztime=30s -parallel=2
mutations:
	python3 scripts/mutation_test.py
security:
	govulncheck ./...
	cd cmd/pcloud-mcp && govulncheck -scan=module
	gitleaks git --redact --no-banner
check: build test
	go mod verify
	python3 scripts/smoke_test.py ./bin/pcloud-mcp
clean:
	rm -rf bin

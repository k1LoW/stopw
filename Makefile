export GO111MODULE=on

default: test

ci: depsdev test race

test:
	go test ./... -coverprofile=coverage.out -covermode=count

race:
	go test ./... -race

benchmark:
	go test -bench . -benchmem -run Benchmark | octocov-go-test-bench --tee > custom_metrics_benchmark.json

lint:
	golangci-lint run ./...

depsdev:
	go install github.com/Songmu/ghch/cmd/ghch@latest
	go install github.com/Songmu/gocredits/cmd/gocredits@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install github.com/k1LoW/octocov-go-test-bench/cmd/octocov-go-test-bench@latest

prerelease:
	git pull origin main --tag
	go mod tidy
	ghch -w -N ${VER}
	gocredits -w .
	git add CHANGELOG.md CREDITS go.mod go.sum
	git commit -m'Bump up version number'
	git tag ${VER}

prerelease_for_tagpr: depsdev
	gocredits -w .
	git add CHANGELOG.md CREDITS go.mod go.sum

release:
	git push origin main --tag

.PHONY: default test

# Every go command here runs with these two settings:
#
# CGO_ENABLED=0 -- on this Mac, Go 1.22.0 binaries linked with cgo crash at
#   launch ("dyld: missing LC_UUID"). Porchlight is pure Go, and so is its
#   SQLite driver (modernc.org/sqlite), so turning cgo off costs nothing and
#   the binary runs.
# GOTOOLCHAIN=local -- use the installed Go; never download a newer one.
export CGO_ENABLED := 0
export GOTOOLCHAIN := local

.PHONY: run build vet test check

run:
	go run .

build:
	go build -o bin/porchlight .

vet:
	go vet ./...

test:
	go test ./...

check: vet test build

commit := $(shell git rev-parse --short HEAD)
tag := $(shell git describe --exact-match --tags 2>/dev/null)
buildtime := $(shell date -u '+%Y-%m-%d %R')

ldflags := -X 'github.com/kycklingar/PBooru/version.Commit=$(commit)' -X 'github.com/kycklingar/PBooru/version.Tag=$(tag)' -X 'github.com/kycklingar/PBooru/version.BuildTime=$(buildtime)'

all:
	go build -ldflags="$(ldflags)" -o out/pbooru

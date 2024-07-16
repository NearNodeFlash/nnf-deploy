all: fmt vet nnf-deploy

nnf-deploy: cmd/main.go config/config.go go.mod
	CGO_ENABLED=0 go build -o ./nnf-deploy cmd/main.go

.PHONY: fmt
fmt:
	go fmt cmd/main.go
	go fmt ./config/*.go

.PHONY: vet
vet:
	go vet cmd/main.go
	go vet ./config/*.go

.PHONY: test
test:
	CGO_ENABLED=0 ginkgo run -p --vv ./config/...

.PHONY: manifests
manifests: NNF_VERSION ?= $(shell ./tools/git-version-gen)
manifests: clean-manifests
	NNF_VERSION=$(NNF_VERSION) tools/collect-manifests.sh -s kind -d $$PWD/release-manifests-kind -t $$PWD/manifests-kind.tar
	NNF_VERSION=$(NNF_VERSION) tools/collect-manifests.sh -s rabbit -d $$PWD/release-manifests -t $$PWD/manifests.tar

.PHONY: clean-manifests
clean-manifests:
	rm -rf release-manifests-kind manifests-kind.tar
	rm -rf release-manifests manifests.tar


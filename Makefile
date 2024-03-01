all: fmt vet nnf-deploy

nnf-deploy: cmd/main.go config/config.go
	go build -o ./nnf-deploy cmd/main.go

.PHONY: fmt
fmt:
	go fmt cmd/main.go
	go fmt ./config/*.go

.PHONY: vet
vet:
	go vet cmd/main.go

.PHONY: test
test:
	ginkgo run -p --vv ./config/...

.PHONY: manifests
manifests:
	tools/collect-manifests.sh -s kind -d $$PWD/release-manifests-kind -t $$PWD/manifests-kind.tar
	tools/collect-manifests.sh -s rabbit -d $$PWD/release-manifests -t $$PWD/manifests.tar

.PHONY: clean-manifests
clean-manifests:
	rm -rf ~+/release-manifests-kind ~+/manifests-kind.tar
	rm -rf ~+/release-manifests ~+/manifests.tar


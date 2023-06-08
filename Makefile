all: fmt vet nnf-deploy

nnf-deploy: main.go
	go build

.PHONY: fmt
fmt:
	go fmt ./main.go
	go fmt ./config/*.go
	go fmt ./test/...

.PHONY: vet
vet:
	go vet ./main.go
	go vet ./test/...

.PHONY: test
test:
	ginkgo run -p --vv ./config/...

.PHONY: int-test
int-test:
	ginkgo run -p --vv ./test/...

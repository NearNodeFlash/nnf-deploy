all: fmt vet nnf-deploy

nnf-deploy: main.go
	go build

.PHONY: fmt
fmt:
	go fmt ./main.go
	go fmt ./config/*.go

.PHONY: vet
vet:
	go vet ./main.go

.PHONY: test
test:
	ginkgo run -p --vv ./config/...


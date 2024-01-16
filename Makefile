all: fmt vet nnf-deploy

nnf-deploy: cmd/main.go
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


.PHONY: test

test:
	ginkgo run -p --vv ./config/...

int-test:
	ginkgo run -p --vv ./test/...

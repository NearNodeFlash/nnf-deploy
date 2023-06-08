package config_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/NearNodeFlash/nnf-deploy/config"
)

var _ = Describe("Config", func() {
	var tempFile *os.File

	BeforeEach(func() {
		var err error
		tempFile, err = os.CreateTemp("", "tmpsysconfig-")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		os.Remove(tempFile.Name())
	})

	createConfigFile := func(input string) {
		_, err := tempFile.WriteString(input)
		Expect(err).ToNot(HaveOccurred())
		defer tempFile.Close()
	}

	Describe("Verifying Systems Configuration", func() {
		When("multiple systems have the same name", func() {
			var input = `
systems:
  - name: one
    workers: [worker1]
    overlays: [overlay1]
    rabbits:
      rabbit-node-1: {0: "compute-1"}
  - name: one
    workers: [worker2]
    overlays: [overlay2]
    rabbits:
      rabbit-node-2: {0: "compute-2"}
`
			It("should error", func() {
				createConfigFile(input)
				_, err := config.ReadConfig(tempFile.Name())
				Expect(err).To(HaveOccurred())
			})
		})

		When("multiple systems have the same alias", func() {
			var input = `
systems:
  - name: one
    aliases: [1]
    workers: [worker1]
    overlays: [overlay1]
    rabbits:
      rabbit-node-1: {0: "compute-1"}
  - name: two
    aliases: [1, 2]
    workers: [worker2]
    overlays: [overlay2]
    rabbits:
      rabbit-node-2: {0: "compute-2"}
`
			It("should error", func() {
				createConfigFile(input)
				_, err := config.ReadConfig(tempFile.Name())
				Expect(err).To(HaveOccurred())
			})
		})

		When("A system has no aliases", func() {
			var input = `
systems:
  - name: no-alias
    overlays: [overlay2]
    workers: [worker2]
    rabbits:
      rabbit-node-2: {0: "compute-2"}
`
			It("should not error", func() {
				createConfigFile(input)
				_, err := config.ReadConfig(tempFile.Name())
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("A system has no workers", func() {
			var input = `
systems:
  - name: no-workers
    overlays: [overlay2]
    rabbits:
      rabbit-node-2: {0: "compute-2"}
`
			It("should error", func() {
				createConfigFile(input)
				_, err := config.ReadConfig(tempFile.Name())
				Expect(err).To(HaveOccurred())
			})
		})

		When("A system has no overlays", func() {
			var input = `
systems:
  - name: no-workers
    workers: [worker2]
    rabbits:
      rabbit-node-2: {0: "compute-2"}
`
			It("should error", func() {
				createConfigFile(input)
				_, err := config.ReadConfig(tempFile.Name())
				Expect(err).To(HaveOccurred())
			})
		})

		When("A system declares a rabbit node twice", func() {
			var input = `
systems:
  - name: two-rabbits
    workers: [worker2]
    overlays: [overlay2]
    rabbits:
      rabbit-node-2: {0: "compute-node-2"}
      rabbit-node-2: {0: "compute-node-3"}
`
			It("should error", func() {
				createConfigFile(input)
				_, err := config.ReadConfig(tempFile.Name())
				Expect(err).To(HaveOccurred())
			})
		})

		When("A system has no rabbit nodes", func() {
			var input = `
systems:
  - name: no-rabbits
    workers: [worker2]
    overlays: [overlay2]
`
			It("should error", func() {
				createConfigFile(input)
				_, err := config.ReadConfig(tempFile.Name())
				Expect(err).To(HaveOccurred())
			})
		})

		When("A system declares a compute node twice", func() {
			var input = `
systems:
  - name: two-computes
    workers: [worker2, worker3]
    overlays: [overlay2]
    rabbits:
      rabbit-node-2: {0: "compute-2"}
      rabbit-node-3: {0: "compute-2"}
`
			It("should error", func() {
				createConfigFile(input)
				_, err := config.ReadConfig(tempFile.Name())
				Expect(err).To(HaveOccurred())
			})
		})

		When("A rabbit node has no compute nodes", func() {
			var input = `
systems:
  - name: two-computes
    workers: [worker2, worker3]
    overlays: [overlay2]
    rabbits:
      rabbit-node-2: {}
      rabbit-node-3: {}
`
			It("should not error", func() {
				createConfigFile(input)
				_, err := config.ReadConfig(tempFile.Name())
				Expect(err).ToNot(HaveOccurred())
			})
		})

	})
})

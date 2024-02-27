/*
 * Copyright 2023-2024 Hewlett Packard Enterprise Development LP
 * Other additional copyright holders may be indicated within.
 *
 * The entirety of this work is licensed under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 *
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
    systemConfiguration: config/systemconfiguration-kind.yaml
  - name: one
    workers: [worker2]
    overlays: [overlay2]
    systemConfiguration: config/systemconfiguration-kind.yaml
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
    systemConfiguration: config/systemconfiguration-kind.yaml
  - name: two
    aliases: [1, 2]
    overlays: [overlay2]
    systemConfiguration: config/systemconfiguration-kind.yaml
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
    systemConfiguration: config/systemconfiguration-kind.yaml
`
			It("should not error", func() {
				createConfigFile(input)
				_, err := config.ReadConfig(tempFile.Name())
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("A system has no overlays", func() {
			var input = `
systems:
  - name: no-workers
    systemConfiguration: config/systemconfiguration-kind.yaml
`
			It("should error", func() {
				createConfigFile(input)
				_, err := config.ReadConfig(tempFile.Name())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

var _ = Describe("SystemConfiguration", func() {

	It("walks the computes for each rabbit in the CR", func() {
		const crPath = "./systemconfiguration-kind.yaml"
		data, err := config.ReadSystemConfigurationCR(crPath)
		Expect(err).ToNot(HaveOccurred())

		perRabbit := data.RabbitsAndComputes()
		rabbit0computes := config.ComputesList{"compute-01", "compute-02", "compute-03"}
		rabbit1computes := config.ComputesList{"compute-04"}

		Expect(perRabbit).To(HaveLen(2))
		for k, v := range perRabbit {
			if k == "kind-worker2" {
				Expect(v).Should(ConsistOf(rabbit0computes))
			} else if k == "kind-worker3" {
				Expect(v).Should(ConsistOf(rabbit1computes))
			} else {
				Expect(v).To(Equal("unknown"))
			}
		}
	})

	It("allows a rabbit to not have computes in the CR", func() {
		const crPath = "./test-files/systemconfiguration-no-computes.yaml"

		data, err := config.ReadSystemConfigurationCR(crPath)
		Expect(err).ToNot(HaveOccurred())

		perRabbit := data.RabbitsAndComputes()

		Expect(perRabbit).To(HaveLen(2))
		for k, v := range perRabbit {
			if k == "kind-worker2" {
				Expect(v).Should(BeNil())
			} else if k == "kind-worker3" {
				Expect(v).Should(BeNil())
			} else {
				Expect(v).To(Equal("unknown"))
			}
		}
	})

	It("allows external computes", func() {
		const crPath = "./systemconfiguration-htx-tds.yaml"
		data, err := config.ReadSystemConfigurationCR(crPath)
		Expect(err).ToNot(HaveOccurred())

		externalComputes := data.ExternalComputes()

		Expect(externalComputes).To(HaveLen(1))
		Expect(externalComputes[0] == "texas-lustre")
	})
})

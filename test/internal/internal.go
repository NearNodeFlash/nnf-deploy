/*
 * Copyright 2023 Hewlett Packard Enterprise Development LP
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

package internal

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	nnfv1alpha1 "github.com/NearNodeFlash/nnf-sos/api/v1alpha1"

	"github.com/HewlettPackard/dws/utils/dwdparse"
)

type T struct {
	// Name is the name of your test case. Name gets formulated into the workflow and
	// related objects as part of test execution.
	name string

	// Labels apply to a test case and can be used by the gingko runner to execute
	// test. See more at TODO LINK TO GINGKO LABEL EXAMPLES
	labels []string

	// Decorators are ginkgo decorators that can be applied to an individual test case
	// Supported decorators are...
	//
	// Focus
	//   The Focus decorator will force Ginkgo to run this test case and other test cases with the
	//   Focus decorator while skipping all other test cases. Ginkgo does not considered any test
	//   suite with a programmatic focus decorator as passing the entirety of the test. While
	//   the test suite might pass, the final exit status will be in error.
	//
	// Pending
	//   The Pending decorator will instruct Ginkgo to skip the case. This is useful if a test is
	//   under development, or perhaps is flaky.
	//
	// For more details, see
	decorators []interface{}

	directives []string

	options TOptions
}

// TOptions let you configure things prior to a test running
type TOptions struct {
	// Create a storage profile for the test case
	storageProfile *TStorageProfile
}

type TStorageProfile struct {
	name string
}

func MakeTest(name string, directives ...string) *T {

	// Extract a common set of labels from the directives
	labels := make([]string, 0)
	for _, directive := range directives {
		args, err := dwdparse.BuildArgsMap(directive)
		if err != nil {
			panic(fmt.Sprintf("Test '%s' failed to parse provided directive '%s'", name, directive))
		}

		labels = append(labels, args["command"])

		if len(args["type"]) != 0 {
			labels = append(labels, args["type"])
		}
	}

	return &T{
		name:       name,
		directives: directives,
		labels:     labels,
		decorators: make([]interface{}, 0),
	}
}

func (t *T) WithStorageProfile(name string) *T {
	t.options.storageProfile = &TStorageProfile{
		name: name,
	}

	return t.WithLabels("storage-profile")
}

// To apply a set of labels for a particular test, use the withLables() method. Labels
const (
	Simple = "simple"
)

func (t *T) WithLabels(labels ...string) *T { t.labels = append(t.labels, labels...); return t }

func (t *T) Focused() *T { t.decorators = append(t.decorators, Focus); return t }
func (t *T) Pending() *T { t.decorators = append(t.decorators, Pending); return t }

func (t *T) Name() string { return t.name }

func (t *T) Args() []interface{} {
	args := make([]interface{}, 0)

	if len(t.labels) != 0 {
		args = append(args, Labels(t.labels))
	}

	if len(t.decorators) != 0 {
		args = append(args, t.decorators...)
	}

	return args
}

func (t *T) Labels() interface{} {
	if len(t.labels) == 0 {
		return nil
	}

	return Labels(t.labels)
}

func (t *T) Decorators() []interface{} {
	if len(t.decorators) == 0 {
		return nil
	}

	return t.decorators
}

func (t *T) Prepare(ctx context.Context, k8sClient client.Client) error {
	o := t.options

	if o.storageProfile != nil {
		// Clone the placeholder profile
		placeholder := &nnfv1alpha1.NnfStorageProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "placeholder",
				Namespace: "nnf-system",
			},
		}

		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(placeholder), placeholder)).To(Succeed())

		profile := &nnfv1alpha1.NnfStorageProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.storageProfile.name,
				Namespace: "nnf-system",
			},
		}

		placeholder.Data.DeepCopyInto(&profile.Data)
		profile.Data.Default = false

		Expect(k8sClient.Create(ctx, profile)).To(Succeed())
	}

	return nil
}

func (t *T) Cleanup(ctx context.Context, k8sClient client.Client) error {
	o := t.options

	if t.options.storageProfile != nil {

		profile := &nnfv1alpha1.NnfStorageProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.storageProfile.name,
				Namespace: "nnf-system",
			},
		}

		Expect(k8sClient.Delete(ctx, profile)).To(Succeed())
	}

	return nil
}

func (t *T) WorkflowName() string {
	return strings.ToLower(strings.ReplaceAll(t.name, " ", "-"))
}

// Retrieve the #DW Directives from the test case
func (t *T) WorkflowDirectives() []string {

	for idx, directive := range t.directives {
		args, err := dwdparse.BuildArgsMap(directive)
		Expect(err).NotTo(HaveOccurred())

		// Make each "#DW jobdw name=[name]" unique so there are no collisions running test in parallel
		name := args["name"]
		Expect(name).NotTo(BeNil())

		directive = strings.Replace(directive, "name="+name, "name="+name+"-"+t.WorkflowName(), 1)

		t.directives[idx] = directive
	}

	return t.directives
}

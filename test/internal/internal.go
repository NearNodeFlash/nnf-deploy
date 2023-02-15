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
	"fmt"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dwsv1alpha1 "github.com/HewlettPackard/dws/api/v1alpha1"
	"github.com/HewlettPackard/dws/utils/dwdparse"
)

type T struct {
	// Name is the name of your test case. Name gets formulated into the workflow and
	// related objects as part of test execution.
	name string

	// Directives are the actual #DW directives used to in the workflow.
	directives []string

	// Labels are simply textual tags that can be attached to a particular test case. Labels
	// provide filter capabilities using via the `ginkgo --label-filter=QUERY` flag.
	//
	// For more details on labels, see https://onsi.github.io/ginkgo/#spec-labels
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
	// Serial
	//   The Serial decorator allows the user to mark specs and containers of specs as only eligible
	//   to run in serial. Ginkgo will guarantee that these specs never run in parallel with other specs.
	//
	// For more details on decorators, see https://onsi.github.io/ginkgo/#decorator-reference
	decorators []interface{}

	// Workflow defines the DWS Workflow resource that is the target of the test.
	workflow *dwsv1alpha1.Workflow

	// Options let you modify the test case with a variety of options and customizations
	options TOptions
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

	t := &T{
		name:       name,
		directives: directives,
		labels:     labels,
		decorators: make([]interface{}, 0),
	}

	t.workflow = &dwsv1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.WorkflowName(),
			Namespace: corev1.NamespaceDefault,
		},
		Spec: dwsv1alpha1.WorkflowSpec{
			DesiredState: dwsv1alpha1.StateProposal,
			DWDirectives: t.WorkflowDirectives(),
			JobID:        GinkgoParallelProcess(),
			WLMID:        strconv.Itoa(GinkgoParallelProcess()),
		},
	}

	return t
}

func (t *T) WorkflowName() string {
	return strings.ToLower(strings.ReplaceAll(t.name, " ", "-"))
}

// Retrieve the #DW Directives from the test case
func (t *T) WorkflowDirectives() []string {
	return t.directives
}

func (t *T) Workflow() *dwsv1alpha1.Workflow {
	return t.workflow
}

// To apply a set of labels for a particular test, use the withLables() method. Labels
const (
	Simple = "simple"
)

func (t *T) WithLabels(labels ...string) *T { t.labels = append(t.labels, labels...); return t }

func (t *T) Focused() *T    { t.decorators = append(t.decorators, Focus); return t }
func (t *T) Pending() *T    { t.decorators = append(t.decorators, Pending); return t }
func (t *T) Serialized() *T { t.decorators = append(t.decorators, Serial); return t }

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

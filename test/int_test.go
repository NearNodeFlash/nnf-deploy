/*
 * Copyright 2022 Hewlett Packard Enterprise Development LP
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

package test

import (
	"strconv"

	. "github.com/NearNodeFlash/nnf-deploy/test/internal"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dwsv1alpha1 "github.com/HewlettPackard/dws/api/v1alpha1"
)

var tests = []*T{
	MakeTest("XFS", "#DW jobdw type=xfs name=xfs capacity=1TB").WithLabels(Simple).Focused(),
	MakeTest("GFS2", "#DW jobdw type=gfs2 name=gfs2 capacity=1TB").WithLabels(Simple),
	MakeTest("Lustre", "#DW jobdw type=lustre name=lustre capacity=1TB").WithLabels(Simple).Pending(),

	// Tests that create and use storage profiles
	MakeTest("XFS with Storage Profile", "#DW jobdw type=xfs name=xfs capacity=1TB profile=my-xfs-profile").
		WithStorageProfile("my-xfs-profile").Pending(),
	MakeTest("GFS2 with Storage Profile", "#DW jobdw type=gfs2 name=gfs2 capacity=1TB profile=my-gfs2-profile").
		WithStorageProfile("my-gfs2-profile").Pending(),

	// Test that use data movement directives
}

var _ = Describe("NNF Integration Test", func() {

	for _, test := range tests {
		test := test

		execFn := func() {
			ctx := ctx
			k8sClient := k8sClient

			BeforeAll(func() {
				Expect(ctx).NotTo(BeNil())
				Expect(k8sClient).NotTo(BeNil())

				Expect(test.Setup(ctx, k8sClient)).To(Succeed())
				DeferCleanup(func() { Expect(test.Cleanup(ctx, k8sClient)).To(Succeed()) })
			})

			workflow := &dwsv1alpha1.Workflow{
				ObjectMeta: metav1.ObjectMeta{
					Name:      test.WorkflowName(),
					Namespace: corev1.NamespaceDefault,
				},
				Spec: dwsv1alpha1.WorkflowSpec{
					DesiredState: dwsv1alpha1.StateProposal,
					DWDirectives: test.WorkflowDirectives(),
					JobID:        GinkgoParallelProcess(),
					WLMID:        strconv.Itoa(GinkgoParallelProcess()),
				},
			}

			BeforeAll(func() {
				Expect(test.CreateWorkflow(ctx, k8sClient, workflow)).To(Succeed())
				DeferCleanup(func() { Expect(test.TeardownWorkflow(ctx, k8sClient, workflow)).To(Succeed()) })
			})

			When("Setup", Ordered, func() { test.WorkflowSetup(ctx, k8sClient, workflow) })
			When("DataIn", Ordered, func() { test.WorkflowDataIn(ctx, k8sClient, workflow) })
			When("PreRun", Ordered, func() { test.WorkflowPreRun(ctx, k8sClient, workflow) })
			When("PostRun", Ordered, func() { test.WorkflowPostRun(ctx, k8sClient, workflow) })
			When("DataOut", Ordered, func() { test.WorkflowDataOut(ctx, k8sClient, workflow) })
		}

		// Formulate the test arguments; this effectively results in
		// return []interface{}{ Label(test.Labels...), Ordered, test.Decorators..., execFn }
		args := func() []interface{} {
			args := make([]interface{}, 0)

			if test.Labels() != nil {
				args = append(args, test.Labels())
			}

			// All specs should be ordered. This ensures that when we run the individual
			// states above (When(...Setup, DataIn, PreRun, PostRun, DataOut)), they are
			// executed in the presented order.
			args = append(args, Ordered)

			if test.Decorators() != nil {
				args = append(args, test.Decorators())
			}

			return append(args, execFn)
		}()

		Describe(test.Name(), args...)
	}
})

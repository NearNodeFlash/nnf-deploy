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
	"fmt"
	"strconv"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	dwsv1alpha1 "github.com/HewlettPackard/dws/api/v1alpha1"
)

var tests = []*T{
	MakeTest("XFS", "#DW jobdw type=xfs name=xfs capacity=1TB").WithLabels(Simple),
	MakeTest("GFS2", "#DW jobdw type=gfs2 name=gfs2 capacity=1TB").WithLabels(Simple),

	MakeTest("Lustre", "#DW jobdw type=lustre name=lustre capacity=1TB").WithLabels(Simple).Pending(),

	// Tests that create and use storage profiles
	MakeTest("XFS with Storage Profile", "#DW jobdw type=xfs name=xfs capacity=1TB profile=my-xfs-profile").
		WithStorageProfile("my-xfs-profile"),
	MakeTest("GFS2 with Storage Profile", "#DW jobdw type=gfs2 name=gfs2 capacity=1TB profile=my-gfs2-profile").
		WithStorageProfile("my-gfs2-profile"),
}

var _ = Describe("NNF Integration Test", func() {

	for _, test := range tests {
		test := test

		Describe(test.Name(), append(test.Args(), func() {
			var workflow *dwsv1alpha1.Workflow

			// Setup any test conditions prior to creating the workflow
			BeforeEach(func() {
				Expect(test.Setup(ctx, k8sClient)).To(Succeed())
				DeferCleanup(func() { Expect(test.Cleanup(ctx, k8sClient)).To(Succeed()) })
			})

			BeforeEach(func() {
				workflow = &dwsv1alpha1.Workflow{
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

				Expect(CreateWorkflow(workflow)).To(Succeed())
				DeferCleanup(func() { Expect(TeardownWorkflow(workflow)).To(Succeed()) })
			})

			ReportAfterEach(func(report SpecReport) {
				if report.Failed() {
					AddReportEntry(fmt.Sprintf("Test %s Failed", test.Name()), workflow)
				}
			})

			// Run the workflow from Setup through Teardown
			It("Executes", func() {
				for _, fn := range []func(*dwsv1alpha1.Workflow){Proposal, Setup, DataIn, PreRun, PostRun, DataOut, Teardown} {
					fn(workflow)
				}
			})

		})...)
	}
})

func CreateWorkflow(workflow *dwsv1alpha1.Workflow) error {
	return k8sClient.Create(ctx, workflow)
}

func TeardownWorkflow(workflow *dwsv1alpha1.Workflow) error {
	advanceStateCheckReady(workflow, dwsv1alpha1.StateTeardown)

	return k8sClient.Delete(ctx, workflow)
}

func Proposal(workflow *dwsv1alpha1.Workflow) {
	Eventually(func(g Gomega) bool {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(workflow), workflow)).To(Succeed())
		return workflow.Status.State == dwsv1alpha1.StateProposal && workflow.Status.Ready
	}).Should(BeTrue())
}

func Setup(workflow *dwsv1alpha1.Workflow) {

	// TODO: Move this to a global variable and initialized in the test suite.
	systemConfig := &dwsv1alpha1.SystemConfiguration{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "default", Namespace: corev1.NamespaceDefault}, systemConfig)).To(Succeed())

	By("Assigns Computes")
	{
		// Assign Compute Resources (only if jobdw or persistentdw is present in workflow())
		// create_persistent & destroy_persistent do not need compute resources
		//Expect(directiveBreakdown.Status.Compute).NotTo(BeNil())
		computes := &dwsv1alpha1.Computes{}
		Expect(k8sClient.Get(ctx, ObjectKeyFromObjectReference(workflow.Status.Computes), computes)).To(Succeed())

		Expect(computes.Data).To(HaveLen(0))

		computes.Data = make([]dwsv1alpha1.ComputesData, 0)
		for _, node := range systemConfig.Spec.ComputeNodes {
			computes.Data = append(computes.Data, dwsv1alpha1.ComputesData{Name: node.Name})
			// TODO: Filter out unwanted compute nodes
		}

		Expect(k8sClient.Update(ctx, computes)).To(Succeed())
	}

	By("Assigns Servers")
	{
		for _, directiveBreakdownRef := range workflow.Status.DirectiveBreakdowns {
			directiveBreakdown := &dwsv1alpha1.DirectiveBreakdown{}
			Eventually(func(g Gomega) bool {
				g.Expect(k8sClient.Get(ctx, ObjectKeyFromObjectReference(directiveBreakdownRef), directiveBreakdown)).To(Succeed())
				return directiveBreakdown.Status.Ready
			}).Should(BeTrue())

			Expect(directiveBreakdown.Status.Storage).NotTo(BeNil())
			Expect(directiveBreakdown.Status.Storage.AllocationSets).NotTo(BeEmpty())

			//
			servers := &dwsv1alpha1.Servers{}
			Expect(k8sClient.Get(ctx, ObjectKeyFromObjectReference(directiveBreakdown.Status.Storage.Reference), servers)).To(Succeed())
			Expect(servers.Spec.AllocationSets).To(BeEmpty())

			// Copy the allocation sets from the directive breakdown to the servers resource, assigning servers
			// as storage resources as necessary.

			// TODO We should assign storage nodes based on the current capabilities of the system and the label. For simple file systems
			// like XFS and GFS2, we can use any Rabbit. But for Lustre, we have to watch where we land the MDT/MGT, and ensure those are
			// exclusive to the Rabbit nodes.
			findStorageServers := func(set *dwsv1alpha1.StorageAllocationSet) []dwsv1alpha1.ServersSpecStorage {
				storages := make([]dwsv1alpha1.ServersSpecStorage, len(systemConfig.Spec.StorageNodes))
				for index, node := range systemConfig.Spec.StorageNodes {
					storages[index].Name = node.Name
					storages[index].AllocationCount = len(node.ComputesAccess)
				}

				return storages
			}

			servers.Spec.AllocationSets = make([]dwsv1alpha1.ServersSpecAllocationSet, len(directiveBreakdown.Status.Storage.AllocationSets))
			for index, allocationSet := range directiveBreakdown.Status.Storage.AllocationSets {
				servers.Spec.AllocationSets[index] = dwsv1alpha1.ServersSpecAllocationSet{
					AllocationSize: allocationSet.MinimumCapacity,
					Label:          allocationSet.Label,
					Storage:        findStorageServers(&allocationSet),
				}
			}

			// TODO: If Lustre - we need to identify the MGT and MDT nodes (and combine them if necessary); and we
			//       can't colocate MGT nodes with other lustre's that might be in test.
			//       OST nodes can go anywhere

			Expect(k8sClient.Update(ctx, servers)).To(Succeed())
		}

	}

	By("Advances to Setup State")
	advanceStateCheckReady(workflow, dwsv1alpha1.StateSetup)
}

func DataIn(workflow *dwsv1alpha1.Workflow) {
	By("Advances to DataIn State")
	advanceStateCheckReady(workflow, dwsv1alpha1.StateDataIn)
}

func PreRun(workflow *dwsv1alpha1.Workflow) {
	By("Advances to PreRun State")
	advanceStateCheckReady(workflow, dwsv1alpha1.StatePreRun)
}

func PostRun(workflow *dwsv1alpha1.Workflow) {
	By("Advances to PostRun State")
	advanceStateCheckReady(workflow, dwsv1alpha1.StatePostRun)
}

func DataOut(workflow *dwsv1alpha1.Workflow) {
	By("Advances to DataOut State")
	advanceStateCheckReady(workflow, dwsv1alpha1.StateDataOut)
}

func Teardown(workflow *dwsv1alpha1.Workflow) {
	By("Advances to Teardown State")
	advanceStateCheckReady(workflow, dwsv1alpha1.StateTeardown)
}

// func DataIn...
// func PreRun...
// func PostRun...
// func DataOut...

func advanceStateCheckReady(workflow *dwsv1alpha1.Workflow, state dwsv1alpha1.WorkflowState) {

	Eventually(func() error {
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(workflow), workflow)).Should(Succeed())
		workflow.Spec.DesiredState = state
		return k8sClient.Update(ctx, workflow)
	}).Should(Succeed(), fmt.Sprintf("updates state to '%s'", state))

	Eventually(func() bool {
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(workflow), workflow)).Should(Succeed())
		return workflow.Status.Ready && workflow.Status.State == state
	}).WithTimeout(time.Minute).WithPolling(time.Second).Should(BeTrue(), fmt.Sprintf("wait for ready in state %s", state))
}

func ObjectKeyFromObjectReference(r corev1.ObjectReference) types.NamespacedName {
	return types.NamespacedName{Name: r.Name, Namespace: r.Namespace}
}

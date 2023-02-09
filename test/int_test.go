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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	dwsv1alpha1 "github.com/HewlettPackard/dws/api/v1alpha1"
	nnfv1alpha1 "github.com/NearNodeFlash/nnf-sos/api/v1alpha1"

	"github.com/HewlettPackard/dws/utils/dwdparse"
)

func ObjectKeyFromObjectReference(r corev1.ObjectReference) types.NamespacedName {
	return types.NamespacedName{Name: r.Name, Namespace: r.Namespace}
}

// Goals:
// 1. Proposal, Setup, ... , Teardown
// 2. All file system types, varrying parameters
// 3. Support advanced Ginkgo features
//    a. Randomization --randomize-all
//    b. Parallelization -p
//    c. Labels / Filtering Specs

// ----------------------------------- EXAMPLE ------------------------------------

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
	//   Focus decorater while skipping all other test cases. Ginkgo does not considered any test
	//   suite with a programatic focus decorator as passing the entirity of the test. While
	//   the test suite might pass, the final exit status will be in error.
	//
	// Pending
	//   The Pending decorator will instruct Ginkgo to skip the case. This is useful if a test is
	//   under development, or perhaps is flaky.
	//
	// For more details, see
	decorators []interface{}

	directives []string

	config  TConfig
	options TOptions
}

type TConfig struct {
	// Similar stuff to what dwsutil uses as configuration options. i.e. Target specific nodes
}

type TOptions struct {
	storageProfile *TStorageProfile
	// Other options might include
	// 1. Setting up a LustreFileSystem for nnf-dm to access
	// ?
}

type TStorageProfile struct {
	name string
}

var Tests = []T{
	// Simple XFS Test
	{

		name:   "XFS",
		labels: []string{"xfs", "simple"},
		//decorators: []interface{}{Focus},
		directives: []string{"#DW jobdw type=xfs name=xfs capacity=1TB"},
	},
	// Complex XFS Test with a unique storage profile created as part of the test
	{

		name:       "XFS with custom Storage Profile",
		labels:     []string{"xfs", "storageprofile"},
		decorators: []interface{}{Focus},
		directives: []string{"#DW jobdw type=xfs name=xfs capacity=1TB profile=my-storage-profile"},

		options: TOptions{
			storageProfile: &TStorageProfile{
				name: "my-storage-profile",
			},
		},
	},

	// Example Lustre
	{
		name:   "Lustre Test",
		labels: []string{"lustre"},
		//decorators: []interface{}{Focus},
		directives: []string{"#DW jobdw type=lustre name=lustre capacity=1TB"},
	},
}

var _ = Describe("NNF Integration Test", func() {

	for _, test := range Tests {
		test := test

		execFn := func() {
			//test := test

			BeforeAll(func() {
				Expect(setupTestOptions(test.options)).To(Succeed())
				DeferCleanup(func() { Expect(cleanupTestOptions(test.options)).To(Succeed()) })
			})

			workflow := &dwsv1alpha1.Workflow{
				ObjectMeta: metav1.ObjectMeta{
					Name:      test.workflowName(),
					Namespace: corev1.NamespaceDefault,
				},
				Spec: dwsv1alpha1.WorkflowSpec{
					DesiredState: dwsv1alpha1.StateProposal,
					DWDirectives: test.workflowDirectives(),
					JobID:        GinkgoParallelProcess(),
					WLMID:        strconv.Itoa(GinkgoParallelProcess()),
				},
			}

			BeforeAll(func() {
				Expect(test.createWorkflow(workflow)).To(Succeed())
				DeferCleanup(func() { Expect(test.teardownWorkflow(workflow)).To(Succeed()) })
			})

			When("Setup", Ordered, func() { test.setup(workflow) })
			When("DataIn", Ordered, func() { test.dataIn(workflow) })
			When("PreRun", Ordered, func() { test.preRun(workflow) })
			When("PostRun", Ordered, func() { test.postRun(workflow) })
			When("DataOut", Ordered, func() { test.dataOut(workflow) })
		}

		// Formulate the test arguments; this effectively results in
		// return []interface{}{ Label(test.Labels...), Ordered, test.Decorators..., execFn }
		args := func() []interface{} {
			args := make([]interface{}, 0)

			if len(test.labels) != 0 {
				args = append(args, Label(test.labels...))
			}
			args = append(args, Ordered)
			args = append(args, test.decorators...)
			return append(args, execFn)
		}()

		Describe(test.name, args...)
	}
})

// Helper methods to Setup/Cleanup the various test options
func setupTestOptions(o TOptions) error {

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
				Namespace: corev1.NamespaceDefault,
			},
		}

		placeholder.Data.DeepCopyInto(&profile.Data)
		profile.Data.Default = false

		Expect(k8sClient.Create(ctx, profile)).To(Succeed())
	}

	return nil
}

func cleanupTestOptions(o TOptions) error {

	if o.storageProfile != nil {

		profile := &nnfv1alpha1.NnfStorageProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.storageProfile.name,
				Namespace: corev1.NamespaceDefault,
			},
		}

		Expect(k8sClient.Delete(ctx, profile)).To(Succeed())
	}

	return nil
}

func (t T) workflowName() string {
	return strings.ToLower(strings.ReplaceAll(t.name, " ", "-"))
}

// Retrieve the #DW Directives from the test case
func (t T) workflowDirectives() []string {

	for idx, directive := range t.directives {
		args, err := dwdparse.BuildArgsMap(directive)
		Expect(err).NotTo(HaveOccurred())

		// Make each "#DW jobdw name=[name]" unique so there are no collisions running test in parallel
		name := args["name"]
		Expect(name).NotTo(BeNil())

		directive = strings.Replace(directive, "name="+name, "name="+name+"-"+t.workflowName(), 1)

		t.directives[idx] = directive
	}

	return t.directives
}

func (t T) createWorkflow(workflow *dwsv1alpha1.Workflow) error {
	Expect(k8sClient.Create(ctx, workflow)).To(Succeed())

	Eventually(func(g Gomega) bool {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(workflow), workflow)).To(Succeed())
		return workflow.Status.State == dwsv1alpha1.StateProposal && workflow.Status.Ready
	}).Should(BeTrue())

	return nil
}

// Helper methods do all the heavy lifting for the test case
func (t T) setup(workflow *dwsv1alpha1.Workflow) {

	// TODO: Move this to a global variable and initialized in the test suite.
	systemConfig := &dwsv1alpha1.SystemConfiguration{}
	It("Gets System Configuration", func() {
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "default", Namespace: corev1.NamespaceDefault}, systemConfig)).To(Succeed())
	})

	It("Assigns Computes", func() {
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
	})

	It("Assigns Servers", func() {

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

	})

	It("Advances to Setup State", func() {
		advanceStateCheckReady(workflow, dwsv1alpha1.StateSetup)
	})
}

func (t T) dataIn(workflow *dwsv1alpha1.Workflow) {
	It("Advances to DataIn State", func() {
		advanceStateCheckReady(workflow, dwsv1alpha1.StateDataIn)
	})
}

func (t T) preRun(workflow *dwsv1alpha1.Workflow) {
	It("Advances to PreRun State", func() {
		advanceStateCheckReady(workflow, dwsv1alpha1.StatePreRun)
	})
}

func (t T) postRun(workflow *dwsv1alpha1.Workflow) {
	It("Advances to PostRun State", func() {
		advanceStateCheckReady(workflow, dwsv1alpha1.StatePostRun)
	})
}

func (t T) dataOut(workflow *dwsv1alpha1.Workflow) {
	It("Advances to DataOut State", func() {
		advanceStateCheckReady(workflow, dwsv1alpha1.StateDataOut)
	})
}

// func DataIn...
// func PreRun...
// func PostRun...
// func DataOut...

func (t T) teardownWorkflow(workflow *dwsv1alpha1.Workflow) error {
	advanceStateCheckReady(workflow, dwsv1alpha1.StateTeardown)

	return k8sClient.Delete(ctx, workflow)
}

func advanceStateCheckReady(workflow *dwsv1alpha1.Workflow, state dwsv1alpha1.WorkflowState) {

	Eventually(func() error {
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(workflow), workflow)).Should(Succeed())
		workflow.Spec.DesiredState = state
		return k8sClient.Update(ctx, workflow)
	}).Should(Succeed(), fmt.Sprintf("updates state to '%s'", state))

	Eventually(func() bool {
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(workflow), workflow)).Should(Succeed())
		return workflow.Status.Ready && workflow.Status.State == state
	}).WithTimeout(time.Minute).Should(BeTrue(), fmt.Sprintf("wait for ready in state %s", state))
}

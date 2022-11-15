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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	dwsv1alpha1 "github.com/HewlettPackard/dws/api/v1alpha1"
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
	name       string
	labels     []string
	decorators []interface{}
	directives []string

	config  TConfig
	options TOptions
}

type TConfig struct {
	// Similar stuff to what dwsutil uses as configuration options
}

type TOptions struct {
	storageProfile *TStorageProfile
	// Other options might include
	// 1. Setting up a LustreFileSystem for nnf-dm to access
	// ?
}

type TStorageProfile struct {
	name       string
	parameters string
}

var Tests = []T{
	// Simple XFS Test
	{

		name:       "Simple XFS Test",
		labels:     []string{"xfs", "simple"},
		directives: []string{"#DW jobdw type=xfs name=xfs capacity=1TB"},
	},
	// Complex XFS Test with a unique storage profile created as part of the test
	{

		name:       "Complex XFS Test",
		labels:     []string{"xfs", "complex", "storageprofiles"},
		directives: []string{"#DW jobdw type=xfs name=xfs capacity=1TB storage_profile=my-storage-profile"},

		options: TOptions{
			storageProfile: &TStorageProfile{
				name:       "my-storage-profile",
				parameters: "some example from dean's confluence page",
			},
		},
	},
	// Example Lustre
	{
		name:       "Lustre Test",
		labels:     []string{"lustre"},
		decorators: []interface{}{},
		directives: []string{"#DW jobdw type=lustre name=lustre capacity=1TB"},
		config:     TConfig{},
		options:    TOptions{},
	},
}

var _ = Describe("NNF Integration Test", func() {

	for _, test := range Tests {
		test := test

		execFn := func() {
			BeforeEach(func() {
				Expect(SetupTestOptions(test.options)).Should(Succeed())
				DeferCleanup(func() { Expect(CleanupTestOptions(test.options)).Should(Succeed()) })
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

			BeforeEach(func() {
				Expect(CreateWorkflow(workflow)).Should(Succeed())
				DeferCleanup(func() { Expect(TeardownWorkflow(workflow)).Should(Succeed()) })
			})

			When("Setup", Ordered, func() { SetupWorkflow(workflow) })
			When("DataIn", Ordered, func() {})
			When("PreRun", func() {})
			When("PostRun", func() {})
			When("DataOut", func() {})
		}

		// Formulate the test arguments; this effectively results in
		// return []interface{}{ Label(test.Labels...), Ordered, test.Decorators..., execFn }
		args := func() []interface{} {
			args := []interface{}{Label(test.labels...), Ordered}
			args = append(args, test.decorators...)
			return append(args, execFn)
		}()

		Describe(test.name, args...)
	}
})

func (t T) WorkflowName() string {
	return strings.ReplaceAll(t.name, " ", "-")
}

// Retrieve the #DW Directives from the test case
func (t T) WorkflowDirectives() []string {

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

// Helper methods to Setup/Cleanup the various test options
func SetupTestOptions(o TOptions) error   { return nil }
func CleanupTestOptions(o TOptions) error { return nil }

func CreateWorkflow(workflow *dwsv1alpha1.Workflow) error {
	Expect(k8sClient.Create(ctx, workflow)).Should(Succeed())

	Eventually(func(g Gomega) bool {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(workflow), workflow)).Should(Succeed())
		return workflow.Status.State == dwsv1alpha1.StateProposal && workflow.Status.Ready
	}).Should(BeTrue())

	return nil
}

// Helper methods do all the heavy lifting for the test case
func SetupWorkflow(workflow *dwsv1alpha1.Workflow) {

	systemConfig := &dwsv1alpha1.SystemConfiguration{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "default", Namespace: corev1.NamespaceDefault}, systemConfig)).Should(Succeed())

	It("Assigns Computes", func() {
		// Assign Compute Resources (only if jobdw or persistentdw is present in workflow())
		// create_persistent & destroy_persistent do not need compute resources
		//Expect(directiveBreakdown.Status.Compute).NotTo(BeNil())
		computes := &dwsv1alpha1.Computes{}
		Expect(k8sClient.Get(ctx, ObjectKeyFromObjectReference(workflow.Status.Computes), computes)).Should(Succeed())

		Expect(computes.Data).To(HaveLen(0))

		computes.Data = make([]dwsv1alpha1.ComputesData, 0)
		for _, node := range systemConfig.Spec.ComputeNodes {
			computes.Data = append(computes.Data, dwsv1alpha1.ComputesData{Name: node.Name})
			// TODO: Filter out unwanted compute nodes
		}

		Expect(k8sClient.Update(ctx, computes)).Should(Succeed())
	})

	It("Assigns Servers", func() {

		for _, directiveBreakdownRef := range workflow.Status.DirectiveBreakdowns {
			directiveBreakdown := &dwsv1alpha1.DirectiveBreakdown{}
			Expect(k8sClient.Get(ctx, ObjectKeyFromObjectReference(directiveBreakdownRef), directiveBreakdown)).Should(Succeed())

			Expect(directiveBreakdown.Status.Ready).To(BeTrue())

			// Assign Rabbit Resources
			Expect(directiveBreakdown.Status.Storage).NotTo(BeNil())

			// If Lustre, there should be 3 allocation sets unless combined mgtmdt is set in the storage profile. Otherwise 1.
			// TODO: Lustre
			Expect(directiveBreakdown.Status.Storage.AllocationSets).To(HaveLen(1))

			servers := &dwsv1alpha1.Servers{}
			Expect(k8sClient.Get(ctx, ObjectKeyFromObjectReference(directiveBreakdown.Status.Storage.Reference), servers))
			Expect(servers.Spec.AllocationSets).To(HaveLen(1))

			storage := make([]dwsv1alpha1.ServersSpecStorage, 0)
			for _, node := range systemConfig.Spec.StorageNodes {
				storage = append(storage, dwsv1alpha1.ServersSpecStorage{
					Name:            node.Name,
					AllocationCount: len(node.ComputesAccess),
				})
			}

			allocationSet := directiveBreakdown.Status.Storage.AllocationSets[0]

			servers.Spec.AllocationSets = []dwsv1alpha1.ServersSpecAllocationSet{
				{
					AllocationSize: allocationSet.MinimumCapacity,
					Label:          allocationSet.Label,
					Storage:        storage,
				},
			}

			// TODO: If Lustre - we need to identify the MGT and MDT nodes (and combine them if necessary); and we
			//       can't colocate MGT nodes with other lustre's that might be in test.
			//       OST nodes can go anywhere

			Expect(k8sClient.Update(ctx, servers)).Should(Succeed())
		}

	})

	It("Advances to Setup State", func() {
		AdvanceStateCheckReady(workflow, dwsv1alpha1.StateSetup)
	})
}

// func DataIn...
// func PreRun...
// func PostRun...
// func DataOut...
func TeardownWorkflow(workflow *dwsv1alpha1.Workflow) error { return nil }

func AdvanceStateCheckReady(workflow *dwsv1alpha1.Workflow, state dwsv1alpha1.WorkflowState) {

	Eventually(func() error {
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(workflow), workflow)).Should(Succeed())
		workflow.Spec.DesiredState = state
		return k8sClient.Update(ctx, workflow)
	}).Should(Succeed(), fmt.Sprintf("Updates the Desired State '%s'", state))

	Eventually(func() bool {
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(workflow), workflow)).Should(Succeed())
		return workflow.Status.Ready && workflow.Status.State == state
	}).Should(BeTrue(), fmt.Sprintf("State '%s' Ready", state))
}

// ----------------------------------- OLD ------------------------------------

// Plan:
// 1. Define list of #DWs, ginkgo labels, ginkgo decorators, and options in a simple file format (yaml/plaintext?)
// 2. Write a go generator that generates the _test.go file **RUN AS PRE-COMMIT CHECK**
//    a. Each and every test follows the same format
//
//  Describe("Test A", Label("Test A Labels"), Ordered | MyDecorators, func() {
//
//    options := &{ Your Test Options Go Here } // i.e. Create Storage Profile
//
//    BeforeEach(func() {
//      SetupTestOptions(options)
//      DeferCleanup(func() { CleanupTestOptions(options)}) })
//    })
//
//    cfg := &{ Your Test Configuration }
//
//    workflow := &{ Name: "Test A", dws: []string { "#DW name=test_a" }}
//
//    Describe("Create Workflow", func()   { CreateWorkflow(workflow, cfg) })
//    Describe("Setup Workflow", func()    { SetupWorkflow(workflow, cfg) })
//    ...
//    Describe("Teardown Workflow", func() { TeardownWorkflow(workflow, cfg) })
//
//  })
//
//  Notes:
//    Describe() allows tests to be randomized and parallelized
//    Label("") allows for filtering of tests based on labels
//    Ordered decorator ensures specs are run sequentially
//    MyDecoraters permits you to add ginkgo's built in decorators Pending, Focused, Skipped
//
// 3. Helper functions (CreateWorkflow, SetupWorkflow, ...) all are wise enough
//    to handle the different workflow types.

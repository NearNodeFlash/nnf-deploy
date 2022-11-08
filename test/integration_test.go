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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	dwsv1alpha1 "github.com/HewlettPackard/dws/api/v1alpha1"
)

func ObjectKeyFromObjectReference(r corev1.ObjectReference) types.NamespacedName {
	return types.NamespacedName{Name: r.Name, Namespace: r.Namespace}
}

var _ = Describe("NNF Integration Test", Ordered, Pending, func() {
	var wf *dwsv1alpha1.Workflow

	It("Creates a Workflow", func() {
		wf = &dwsv1alpha1.Workflow{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: corev1.NamespaceDefault,
			},
			Spec: dwsv1alpha1.WorkflowSpec{
				DesiredState: dwsv1alpha1.StateProposal,
				DWDirectives: []string{
					"#DW jobdw type=gfs2 capacity=10GB name=gfs2",
				},
				UserID:  0,
				GroupID: 0,
			},
		}

		Expect(k8sClient.Create(ctx, wf)).Should(Succeed())
	})

	It("Assigns Servers", func() {
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(wf), wf)).Should(Succeed())
			return wf.Status.Ready
		}).Should(BeTrue())

		for _, dbRef := range wf.Status.DirectiveBreakdowns {
			db := &dwsv1alpha1.DirectiveBreakdown{}
			Expect(k8sClient.Get(ctx, ObjectKeyFromObjectReference(dbRef), db)).Should(Succeed())

		}
	})

})

// Goals:
// 1. Proposal, Setup, ... , Teardown
// 2. All file system types, varrying parameters
// 3. Support advanced Ginkgo features
//    a. Randomization --randomize-all
//    b. Parallelization -p
//    c. Labels / Filtering Specs

// ----------------------------------- EXAMPLE ------------------------------------

type T struct {
	Name       string
	Labels     []string
	Decorators []interface{}
	Directives []string

	Config  TConfig
	Options TOptions
}

type TConfig struct {
	// Similar stuff to what dwsutil uses as configuration options
}

type TOptions struct {
	StorageProfile *TStorageProfile
	// Other options might include
	// 1. Setting up a LustreFileSystem for nnf-dm to access
	// ?
}

type TStorageProfile struct {
	Name       string
	Parameters string
}

var Tests = []T{
	// Simple XFS Test
	{

		Name:       "Simple XFS Test",
		Labels:     []string{"xfs", "simple"},
		Directives: []string{"#DW jobdw type=xfs name=xfs"},
	},
	// Complex XFS Test with a unique storage profile created as part of the test
	{

		Name:       "Complex XFS Test",
		Labels:     []string{"xfs", "complex", "storageprofiles"},
		Directives: []string{"#DW jobdw type=xfs storage_profile=my-storage-profile"},

		Options: TOptions{
			StorageProfile: &TStorageProfile{
				Name:       "my-storage-profile",
				Parameters: "some example from dean's confluence page",
			},
		},
	},
}

// Helper methods to Setup/Cleanup the various test options
func SetupTestOptions(o TOptions) error   { return nil }
func CleanupTestOptions(o TOptions) error { return nil }

// Helper methods do all the heavy lifting for the test case
func SetupWorkflow(workflow *dwsv1alpha1.Workflow) {}

// func DataIn...
// func PreRun...
// func PostRun...
// func DataOut...
func TeardownWorkflow(workflow *dwsv1alpha1.Workflow) error { return nil }

var _ = Describe("NNF Integration Test", func() {

	for _, test := range Tests {
		test := test

		Describe(test.Name, Label(test.Labels...), Ordered /*TODO: How to add Pending|Focus|Skip Decorators */, func() {

			BeforeEach(func() {
				Expect(SetupTestOptions(test.Options)).Should(Succeed())
				DeferCleanup(func() { Expect(CleanupTestOptions(test.Options)).Should(Succeed()) })
			})

			workflow := &dwsv1alpha1.Workflow{
				ObjectMeta: metav1.ObjectMeta{
					Name:      test.Name,
					Namespace: corev1.NamespaceDefault,
				},
				Spec: dwsv1alpha1.WorkflowSpec{
					DesiredState: dwsv1alpha1.StateProposal,
					DWDirectives: test.Directives,
				},
			}

			BeforeEach(func() {
				Expect(k8sClient.Create(ctx, workflow)).Should(Succeed())
				DeferCleanup(func() { Expect(TeardownWorkflow(workflow)).Should(Succeed()) })
			})

			When("Setup", func() { SetupWorkflow(workflow) })
			When("DataIn", func() {})
			When("PreRun", func() {})
			When("PostRun", func() {})
			When("DataOut", func() {})
		})
	}
})

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

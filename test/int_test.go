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

package test

import (
	"fmt"

	. "github.com/NearNodeFlash/nnf-deploy/test/internal"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	dwsv1alpha1 "github.com/HewlettPackard/dws/api/v1alpha1"
)

var tests = []*T{
	// Examples:
	//
	// Mark a test case as Focused(). Ginkgo will only run tests that have the Focus decorator.
	//   MakeTest("Focused", "#DW ...").Focused(),
	//
	// Mark a test case as Pending(). Ginkgo will not run any tests that have the Pending decorator
	//   MakeTest("Pending", "#DW ...").Pending()
	//
	// Mark a test case so it will stop after the workflow achieves the desired state of PreRun
	//   MakeTest("Stop After", "#DW ...").StopAfter(wsv1alpha1.StatePreRun),
	//
	// Duplicate a test case 20 times.
	//   DuplicateTest(
	//      MakeTest("XFS", "#DW jobdw type=xfs name=xfs capacity=1TB"),
	//      20,
	//   ),

	MakeTest("XFS", "#DW jobdw type=xfs name=xfs capacity=1TB").WithLabels(Simple),
	MakeTest("GFS2", "#DW jobdw type=gfs2 name=gfs2 capacity=1TB").WithLabels(Simple).Pending(),
	MakeTest("Lustre", "#DW jobdw type=lustre name=lustre capacity=1TB").WithLabels(Simple).Pending(),

	DuplicateTest(
		MakeTest("XFS", "#DW jobdw type=xfs name=xfs capacity=1TB").Pending(), // Will fail for Setup() exceeding time limit; needs investigation
		5,
	),

	// Storage Profiles
	MakeTest("XFS with Storage Profile",
		"#DW jobdw type=xfs name=xfs-storage-profile capacity=1TB profile=my-xfs-storage-profile").
		WithStorageProfile(),
	MakeTest("GFS2 with Storage Profile",
		"#DW jobdw type=gfs2 name=gfs2-storage-profile capacity=1TB profile=my-gfs2-storage-profile").
		WithStorageProfile().
		Pending(),

	// Persistent
	MakeTest("Persistent Lustre",
		"#DW create_persistent type=lustre name=persistent-lustre capacity=1TB").
		AndCleanupPersistentInstance().
		Serialized(),

	// Data Movement
	MakeTest("XFS with Data Movement",
		"#DW jobdw type=xfs name=xfs-data-movement capacity=1TB",
		"#DW copy_in source=/lus/global/test.in destination=$DW_JOB_xfs-data-movement/",    // TODO: Create a file "test.in" in the global lustre directory
		"#DW copy_out source=$DW_JOB_xfs-data-movement/test.out destination=/lus/global/"). // TODO: Validate file "test.out" in the global lustre directory
		WithPersistentLustre("xfs-data-movement-lustre-instance").                          // Manage a persistent Lustre instance as part of the test
		WithGlobalLustreFromPersistentLustre("/lus/global").
		Serialized().
		Pending(),

	MakeTest("GFS2 with Containers (BLAKE)",
		"#DW jobdw type=gfs2 name=gfs2-with-containers capacity=1TB",
		"#DW container name=gfs2-with-containers profile=TODO DW_JOB_gfs2-with-containers",
	).Pending(),
}

var _ = Describe("NNF Integration Test", func() {

	iterator := TestIterator(tests)
	for t := iterator.Next(); t != nil; t = iterator.Next() {

		t := t

		Describe(t.Name(), append(t.Args(), func() {

			// Prepare any necessary test conditions prior to creating the workflow
			BeforeEach(func() {
				Expect(t.Prepare(ctx, k8sClient)).To(Succeed())
				DeferCleanup(func() { Expect(t.Cleanup(ctx, k8sClient)).To(Succeed()) })
			})

			// Create the workflow and delete it on cleanup
			BeforeEach(func() {
				workflow := t.Workflow()

				Expect(k8sClient.Create(ctx, workflow)).To(Succeed())

				DeferCleanup(func(context SpecContext) {
					if t.ShouldTeardown() {
						// TODO: Ginkgo's `--fail-fast` option still seems to execute DeferCleanup() calls
						//       See if this is by design or if we might need to move this to an AfterEach()
						if !context.SpecReport().Failed() {
							AdvanceStateAndWaitForReady(ctx, k8sClient, workflow, dwsv1alpha1.StateTeardown)

							Expect(k8sClient.Delete(ctx, workflow)).To(Succeed())
						}
					}
				})
			})

			// Report additional workflow data for each failed test
			ReportAfterEach(func(report SpecReport) {
				if report.Failed() {
					workflow := t.Workflow()
					AddReportEntry(fmt.Sprintf("Workflow '%s' Failed", workflow.Name), workflow.Status)
				}
			})

			// Run the workflow from Setup through Teardown
			It("Executes", func() { t.Execute(ctx, k8sClient) })

		})...)
	}
})

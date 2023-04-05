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

	// Containers
	MakeTest("GFS2 with Containers",
		"#DW jobdw type=gfs2 name=gfs2-with-containers capacity=100GB",
		"#DW container name=gfs2-with-containers profile=example-success DW_JOB_foo-local-storage=gfs2-with-containers").
		WithPermissions(1000, 2000),
	MakeTest("GFS2 with Containers Root",
		"#DW jobdw type=gfs2 name=gfs2-with-containers-root capacity=100GB",
		"#DW container name=gfs2-with-containers-root profile=example-success DW_JOB_foo-local-storage=gfs2-with-containers-root").
		WithPermissions(0, 0),
	MakeTest("GFS2 with MPI Containers",
		"#DW jobdw type=gfs2 name=gfs2-with-containers-mpi capacity=100GB",
		"#DW container name=gfs2-with-containers-mpi profile=example-mpi DW_JOB_foo-local-storage=gfs2-with-containers-mpi").
		WithPermissions(1050, 2050),
	MakeTest("GFS2 with MPI Containers Root",
		"#DW jobdw type=gfs2 name=gfs2-with-containers-mpi-root capacity=100GB",
		"#DW container name=gfs2-with-containers-mpi-root profile=example-mpi DW_JOB_foo-local-storage=gfs2-with-containers-mpi-root").
		WithPermissions(1050, 2050),

	// These two test should fail as xfs/raw filesystems are not supported for containers
	MakeTest("XFS with Containers",
		"#DW jobdw type=xfs name=xfs-with-containers capacity=100GB",
		"#DW container name=xfs-with-containers profile=example-success DW_JOB_foo-local-storage=xfs-with-containers").
		ExpectError(dwsv1alpha1.StateProposal),
	MakeTest("Raw with Containers",
		"#DW jobdw type=raw name=raw-with-containers capacity=100GB",
		"#DW container name=raw-with-containers profile=example-success DW_JOB_foo-local-storage=raw-with-containers").
		ExpectError(dwsv1alpha1.StateProposal),

	// TODO: The timing on these needs some work, hence Pending()
	MakeTest("GFS2 and Lustre with Containers",
		"#DW jobdw name=containers-local-storage type=gfs2 capacity=100GB",
		"#DW persistentdw name=containers-persistent-storage",
		"#DW container name=gfs2-lustre-with-containers profile=example-success DW_JOB_foo-local-storage=containers-local-storage DW_PERSISTENT_foo-persistent-storage=containers-persistent-storage").
		WithPersistentLustre("containers-persistent-storage").
		WithPermissions(1050, 2050).
		Pending(),
	MakeTest("GFS2 and Lustre with Containers MPI",
		"#DW jobdw name=containers-local-storage-mpi type=gfs2 capacity=100GB",
		"#DW persistentdw name=containers-persistent-storage-mpi",
		"#DW container name=gfs2-lustre-with-containers-mpi profile=example-mpi DW_JOB_foo-local-storage=containers-local-storage-mpi DW_PERSISTENT_foo-persistent-storage=containers-persistent-storage-mpi").
		WithPersistentLustre("containers-persistent-storage-mpi").
		WithPermissions(1050, 2050).
		Pending(),
}

var _ = Describe("NNF Integration Test", func() {

	iterator := TestIterator(tests)
	for t := iterator.Next(); t != nil; t = iterator.Next() {

		// Note that you must assign a copy of the loop variable to a local variable - otherwise
		// the closure will capture the mutating loop variable and all the specs will run against
		// the last element in the loop. It is idiomatic to give the local copy the same name as
		// the loop variable.
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

				By(fmt.Sprintf("Creating workflow '%s'", workflow.Name))
				Expect(k8sClient.Create(ctx, workflow)).To(Succeed())

				DeferCleanup(func(context SpecContext) {
					if t.ShouldTeardown() {
						// TODO: Ginkgo's `--fail-fast` option still seems to execute DeferCleanup() calls
						//       See if this is by design or if we might need to move this to an AfterEach()
						if !context.SpecReport().Failed() {
							t.AdvanceStateAndWaitForReady(ctx, k8sClient, workflow, dwsv1alpha1.StateTeardown)

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

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

	. "github.com/NearNodeFlash/nnf-deploy/test/internal"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	dwsv1alpha1 "github.com/HewlettPackard/dws/api/v1alpha1"
)

var tests = []*T{
	MakeTest("XFS", "#DW jobdw type=xfs name=xfs capacity=1TB").WithLabels(Simple),
	MakeTest("GFS2", "#DW jobdw type=gfs2 name=gfs2 capacity=1TB").WithLabels(Simple).Pending(),

	MakeTest("Lustre", "#DW jobdw type=lustre name=lustre capacity=1TB").WithLabels(Simple).Pending(),

	// Tests that create and use storage profiles
	MakeTest("XFS with Storage Profile", "#DW jobdw type=xfs name=xfsStorageProfile capacity=1TB profile=my-xfs-profile").
		WithStorageProfile("my-xfs-profile"),
	MakeTest("GFS2 with Storage Profile", "#DW jobdw type=gfs2 name=gfs2 capacity=1TB profile=my-gfs2-profile").
		WithStorageProfile("my-gfs2-profile").Pending(),

	// Data Movement
	MakeTest("XFS with Data Movement",
		"#DW jobdw type=xfs name=xfs-data-movement capacity=1TB",
		"#DW copy_in source=/lus/global/test.in destination=$JOB_DW_xfs/",
		"#DW copy_out source=$JOB_DW_xfs/test.out destination=/lus/global/").
		WithPersistentLustre("xfs-data-movement-lustre-instance"). // Setup a persistent Lustre instance as part of the test
		WithGlobalLustreFromPersistentLustre("/lus/global", "test.in", "test.out").
		Serialized(),

	MakeTest("GFS2 with Containers (BLAKE)",
		"#DW jobdw type=gfs2 name=gfs2-with-containers capacity=1TB",
		"#DW container name=gfs2-with-containers profile=TODO DW_JOB_gfs2-with-containers",
	).Pending(),
}

var _ = Describe("NNF Integration Test", func() {

	for _, t := range tests {
		t := t

		Describe(t.Name(), append(t.Args(), func() {
			var workflow *dwsv1alpha1.Workflow

			// Prepare any necessary test conditions prior to creating the workflow
			BeforeEach(func() {
				Expect(t.Prepare(ctx, k8sClient)).To(Succeed())
				DeferCleanup(func() { Expect(t.Cleanup(ctx, k8sClient)).To(Succeed()) })
			})

			BeforeEach(func() {
				workflow = t.Workflow()

				Expect(k8sClient.Create(ctx, workflow)).To(Succeed())

				DeferCleanup(func(context SpecContext) {
					// TODO: Ginkgo's `--fail-fast` option still seems to execute DeferCleanup() calls
					//       See if this is by design or if we might need to move this to an AfterEach()
					if !context.SpecReport().Failed() {
						AdvanceStateAndWaitForReady(ctx, k8sClient, workflow, dwsv1alpha1.StateTeardown)

						Expect(k8sClient.Delete(ctx, workflow)).To(Succeed())
					}
				})
			})

			ReportAfterEach(func(report SpecReport) {
				if report.Failed() {
					AddReportEntry(fmt.Sprintf("Workflow '%s' Failed", workflow.Name), workflow.Status)
				}
			})

			// Run the workflow from Setup through Teardown
			It("Executes", func() {
				for _, fn := range []StateHandler{t.Proposal, t.Setup, t.DataIn, t.PreRun, t.PostRun, t.DataOut, t.Teardown} {
					fn(ctx, k8sClient, workflow)
				}
			})

		})...)
	}
})

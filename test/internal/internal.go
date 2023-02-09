package internal

import (
	"context"
	"fmt"
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

func (t *T) Setup(ctx context.Context, k8sClient client.Client) error {
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

func (t *T) CreateWorkflow(ctx context.Context, k8sClient client.Client, workflow *dwsv1alpha1.Workflow) error {
	Expect(k8sClient.Create(ctx, workflow)).To(Succeed())

	Eventually(func(g Gomega) bool {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(workflow), workflow)).To(Succeed())
		return workflow.Status.State == dwsv1alpha1.StateProposal && workflow.Status.Ready
	}).Should(BeTrue())

	return nil
}

func (t *T) TeardownWorkflow(ctx context.Context, k8sClient client.Client, workflow *dwsv1alpha1.Workflow) error {
	advanceStateCheckReady(ctx, k8sClient, workflow, dwsv1alpha1.StateTeardown)

	return k8sClient.Delete(ctx, workflow)
}

// Helper methods do all the heavy lifting for the test case
func (t *T) WorkflowSetup(ctx context.Context, k8sClient client.Client, workflow *dwsv1alpha1.Workflow) {

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
		advanceStateCheckReady(ctx, k8sClient, workflow, dwsv1alpha1.StateSetup)
	})
}

func (t *T) WorkflowDataIn(ctx context.Context, k8sClient client.Client, workflow *dwsv1alpha1.Workflow) {
	It("Advances to DataIn State", func() {
		advanceStateCheckReady(ctx, k8sClient, workflow, dwsv1alpha1.StateDataIn)
	})
}

func (t *T) WorkflowPreRun(ctx context.Context, k8sClient client.Client, workflow *dwsv1alpha1.Workflow) {
	It("Advances to PreRun State", func() {
		advanceStateCheckReady(ctx, k8sClient, workflow, dwsv1alpha1.StatePreRun)
	})
}

func (t *T) WorkflowPostRun(ctx context.Context, k8sClient client.Client, workflow *dwsv1alpha1.Workflow) {
	It("Advances to PostRun State", func() {
		advanceStateCheckReady(ctx, k8sClient, workflow, dwsv1alpha1.StatePostRun)
	})
}

func (t *T) WorkflowDataOut(ctx context.Context, k8sClient client.Client, workflow *dwsv1alpha1.Workflow) {
	It("Advances to DataOut State", func() {
		advanceStateCheckReady(ctx, k8sClient, workflow, dwsv1alpha1.StateDataOut)
	})
}

// func DataIn...
// func PreRun...
// func PostRun...
// func DataOut...

func advanceStateCheckReady(ctx context.Context, k8sClient client.Client, workflow *dwsv1alpha1.Workflow, state dwsv1alpha1.WorkflowState) {

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

func ObjectKeyFromObjectReference(r corev1.ObjectReference) types.NamespacedName {
	return types.NamespacedName{Name: r.Name, Namespace: r.Namespace}
}

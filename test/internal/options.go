package internal

import (
	"context"
	"fmt"

	dwsv1alpha1 "github.com/HewlettPackard/dws/api/v1alpha1"
	"github.com/HewlettPackard/dws/utils/dwdparse"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	lusv1alpha1 "github.com/NearNodeFlash/lustre-fs-operator/api/v1alpha1"
	nnfv1alpha1 "github.com/NearNodeFlash/nnf-sos/api/v1alpha1"
)

// TOptions let you configure things prior to a test running or during test
// execution. Nil values represent no configuration of that type.
type TOptions struct {
	stopAfter         *TStopAfter
	storageProfile    *TStorageProfile
	persistentLustre  *TPersistentLustre
	globalLustre      *TGlobalLustre
	cleanupPersistent *TCleanupPersistentInstance
}

type TStopAfter struct {
	state dwsv1alpha1.WorkflowState
}

// Stop after lets you stop a test after a given state is reached
func (t *T) StopAfter(state dwsv1alpha1.WorkflowState) *T {
	t.options.stopAfter = &TStopAfter{state: state}
	return t
}

func (t *T) ShouldTeardown() bool {
	return t.options.stopAfter == nil
}

type TStorageProfile struct {
	name string
}

// WithStorageProfile will manage a storage profile of of name 'name'
func (t *T) WithStorageProfile() *T {

	for _, directive := range t.directives {
		args, _ := dwdparse.BuildArgsMap(directive)

		if args["command"] == "jobdw" || args["command"] == "create_persistent" {
			if name, found := args["profile"]; found {
				t.options.storageProfile = &TStorageProfile{name: name}
				return t.WithLabels("storage_profile")
			}
		}
	}

	panic(fmt.Sprintf("profile argument required but not found in test '%s'", t.Name()))
}

type TPersistentLustre struct {
	name string

	// Use internal tests to drive the persistent lustre workflow
	create  *T
	destroy *T

	fsName  string
	mgsNids string
}

func (t *T) WithPersistentLustre(name string) *T {
	t.options.persistentLustre = &TPersistentLustre{name: name}
	return t.WithLabels("persistent", "lustre")
}

type TCleanupPersistentInstance struct {
	name string
}

// AndCleanupPersistentInstance will automatically destroy the persistent instance. It is useful
// if you have a create_persistent directive that you wish to destroy after the test has finished.
func (t *T) AndCleanupPersistentInstance() *T {
	for _, directive := range t.directives {
		args, _ := dwdparse.BuildArgsMap(directive)

		if args["command"] == "create_persistent" {
			t.options.cleanupPersistent = &TCleanupPersistentInstance{
				name: args["name"],
			}

			return t
		}
	}

	panic(fmt.Sprintf("create_persistent directive required but not found in test '%s'", t.Name()))
}

type TGlobalLustre struct {
	fsName    string
	mgsNids   string
	mountRoot string

	in  string // Create this file prior copy_in
	out string // Expect this file after copy_out

	persistent *TPersistentLustre // If using a persistent lustre instance as the global lustre
}

func (t *T) WithGlobalLustre(mountRoot string, fsName string, mgsNids string) {
	panic("reference to an existing global lustre instance is not yet supported")
}

// WithGlobalLustreFromPersistentLustre will create a global lustre file system from a persistent lustre file system
func (t *T) WithGlobalLustreFromPersistentLustre(mountRoot string) *T {
	if t.options.persistentLustre == nil {
		panic("Test option requires persistent lustre")
	}

	t.options.globalLustre = &TGlobalLustre{
		persistent: t.options.persistentLustre,
		mountRoot:  mountRoot,
	}

	return t.WithLabels("global_lustre")
}

// Prepare a test with the programmed test options
func (t *T) Prepare(ctx context.Context, k8sClient client.Client) error {
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

	if o.cleanupPersistent != nil {
		// Nothing to do in Prepare()
	}

	if o.persistentLustre != nil {
		// Create a persistent lustre instance all the way to pre-run
		name := o.persistentLustre.name

		o.persistentLustre.create = MakeTest(name+"-create",
			fmt.Sprintf("#DW create_persistent type=lustre name=%s capacity=1TB", name))
		o.persistentLustre.destroy = MakeTest(name+"-destroy",
			fmt.Sprintf("#DW destroy_persistent name=%s", name))

		// Create the persistent lustre instance
		Expect(k8sClient.Create(ctx, o.persistentLustre.create.Workflow())).To(Succeed())
		o.persistentLustre.create.Execute(ctx, k8sClient)

		// TODO: Extract the File System Name and MGSNids from the persistent lustre instance
		o.persistentLustre.fsName = "TODO"
		o.persistentLustre.mgsNids = "TODO"
	}

	if o.globalLustre != nil {

		lustre := &lusv1alpha1.LustreFileSystem{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "global",
				Namespace: corev1.NamespaceDefault,
			},
			Spec: lusv1alpha1.LustreFileSystemSpec{
				Name:      o.globalLustre.fsName,
				MgsNids:   o.globalLustre.mgsNids,
				MountRoot: o.globalLustre.mountRoot,
			},
		}

		if o.globalLustre.persistent != nil {
			lustre.Spec.Name = o.globalLustre.persistent.fsName
			lustre.Spec.MgsNids = o.globalLustre.persistent.mgsNids
		} else {
			panic("reference to an existing global lustre file system is not yet implemented")
		}

		Expect(k8sClient.Create(ctx, lustre)).To(Succeed())

	}

	return nil
}

// Cleanup a test with the programmed test options.
// NOTE: The order in which test options are cleanup is the opposite order of
//
//	their creation to ensure dependencies between options are correct.
func (t *T) Cleanup(ctx context.Context, k8sClient client.Client) error {
	o := t.options

	if o.globalLustre != nil {
		lustre := &lusv1alpha1.LustreFileSystem{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "global",
				Namespace: "nnf-dm-system",
			},
		}

		Expect(k8sClient.Delete(ctx, lustre)).To(Succeed())
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(lustre), lustre)
		}).ShouldNot(Succeed(), "lustre file system resource should delete")
	}

	if o.cleanupPersistent != nil {
		name := o.cleanupPersistent.name

		test := MakeTest(name+"-destroy", fmt.Sprintf("#DW destroy_persistent name=%s", name))
		Expect(k8sClient.Create(ctx, test.Workflow())).To(Succeed())
		test.Execute(ctx, k8sClient)
		Expect(k8sClient.Delete(ctx, test.Workflow())).To(Succeed())
	}

	if o.persistentLustre != nil {

		// Destroy the persistent lustre instance we previously created
		Expect(k8sClient.Create(ctx, o.persistentLustre.destroy.Workflow())).To(Succeed())
		o.persistentLustre.destroy.Execute(ctx, k8sClient)

		Expect(k8sClient.Delete(ctx, o.persistentLustre.create.Workflow())).To(Succeed())
		Expect(k8sClient.Delete(ctx, o.persistentLustre.destroy.Workflow())).To(Succeed())
	}

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

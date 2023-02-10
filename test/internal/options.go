package internal

import (
	"context"
	"fmt"

	dwsv1alpha1 "github.com/HewlettPackard/dws/api/v1alpha1"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	nnfv1alpha1 "github.com/NearNodeFlash/nnf-sos/api/v1alpha1"
)

// TOptions let you configure things prior to a test running or during test
// execution. Nil values represent no configuration of that type.
type TOptions struct {
	storageProfile   *TStorageProfile
	persistentLustre *TPersistentLustre
	globalLustre     *TGlobalLustre
}

type TStorageProfile struct {
	name string
}

// WithStorageProfile will manage a storage profile of of name 'name'
func (t *T) WithStorageProfile(name string) *T {
	t.options.storageProfile = &TStorageProfile{name: name}

	return t.WithLabels("storage-profile")
}

type TPersistentLustre struct {
	name string

	test     *T
	workflow *dwsv1alpha1.Workflow
}

func (t *T) WithPersistentLustre(name string) *T {
	t.options.persistentLustre = &TPersistentLustre{name: name}
	return t.WithLabels("persistent", "lustre")
}

type TGlobalLustre struct {
	persistent *TPersistentLustre

	path string
	in   string // Create this file prior copy_in
	out  string // Expect this file after copy_out
}

// WithGlobalLustreFromPersistentLustre will create a global lustre file system from a persistent lustre file system
func (t *T) WithGlobalLustreFromPersistentLustre(path string, in string, out string) *T {
	if t.options.persistentLustre == nil {
		panic("Test option requires persistent lustre")
	}

	t.options.globalLustre = &TGlobalLustre{
		persistent: t.options.persistentLustre,
		path:       path,
		in:         in,
		out:        out,
	}

	return t.WithLabels("global-lustre")
}

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

	if o.persistentLustre != nil {
		// Create a persistent lustre instance all the way to pre-run
		name := o.persistentLustre.name

		o.persistentLustre.test = MakeTest(name, fmt.Sprintf("#DW create_persistent type=lustre name=%s capacity=1TB", name))

		for _, fn := range []StateHandler{t.Proposal, t.Setup, t.DataIn, t.PreRun} {
			fn(ctx, k8sClient, o.persistentLustre.test.workflow)
		}
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

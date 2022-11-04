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

var _ = Describe("NNF Integration Test", Ordered, func() {
	var wf *dwsv1alpha1.Workflow

	It("Creates a Workflow", func() {
		wf = &dwsv1alpha1.Workflow{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: corev1.NamespaceDefault,
			},
			Spec: dwsv1alpha1.WorkflowSpec{
				DesiredState: dwsv1alpha1.StateProposal.String(),
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

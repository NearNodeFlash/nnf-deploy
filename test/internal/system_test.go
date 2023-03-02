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

package internal

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAuthorizedDevelopers(t *testing.T) {

	names := []string{
		"bryce",
		"bryced",
		"brycedevcich",
		"bryce-d",
		"bryce-devcich",
		"bryce-is-a-pretty-cool-dude-if-you-get-to-know-him",
	}

	for _, name := range names {
		namespaces := corev1.NamespaceList{
			Items: []corev1.Namespace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: name,
					},
				},
			},
		}

		reserved, developer, err := isReserved(&namespaces)
		if err != nil {
			t.Errorf("error %t", err)
		}

		if !reserved {
			t.Errorf("reservation '%s' not found", name)
		}

		t.Logf("reservation '%s' found for '%s'", name, developer)
	}

}

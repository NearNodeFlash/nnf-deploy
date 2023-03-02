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
	"context"
	"fmt"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	TriageNamespaceName = "nnf-system-needs-triage"
)

func IsSystemInNeedOfTriage(ctx context.Context, k8sClient client.Client) bool {

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: TriageNamespaceName}}
	err := k8sClient.Get(ctx, client.ObjectKeyFromObject(ns), ns)

	return !errors.IsNotFound(err)
}

func SetSystemInNeedOfTriage(ctx context.Context, k8sClient client.Client) error {

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: TriageNamespaceName}}
	if err := k8sClient.Create(ctx, ns); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// IsSystemReserved checks if the system under test is reserved by a known developer.
func IsSystemReserved(ctx context.Context, k8sClient client.Client) (bool, string, error) {

	namespaces := &corev1.NamespaceList{}
	if err := k8sClient.List(ctx, namespaces); err != nil {
		return false, "", err
	}

	return isReserved(namespaces)
}

var authorizedDevelopers = []string{
	"Abhishek Girish",
	"Ben Landsteiner",
	"Blake Devcich",
	"Bryce Devcich",
	"Dean Roehrich",
	"Matt Richerson",
	"Tom Albers",
	"Tony Floeder",
	"Tim McCree",
}

func isReserved(namespaces *corev1.NamespaceList) (bool, string, error) {

	// Reservations are of the form "firstName-?(lastName|lastInitial)?"

	for _, developer := range authorizedDevelopers {
		first, last, _ := strings.Cut(strings.ToLower(developer), " ")
		re, err := regexp.Compile(fmt.Sprintf("^(%s-?(%s|%c)?)", first, last, last[0]))
		if err != nil {
			return false, "", err
		}

		for _, ns := range namespaces.Items {
			if re.MatchString(ns.Name) {
				return true, developer, nil
			}
		}

	}

	return false, "", nil
}

// badref - Find invalid ownerReferences in a Kubernetes cluster
// Copyright (C) 2020  John D. Strunk

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type ObjectDescription struct {
	APIVersion string
	Kind       string
	Name       string
	Namespace  string
	types.UID
	OwnerReferences []v1.OwnerReference
	IsNamespaced    bool
}

func (d ObjectDescription) KindNamespaceName() string {
	if d.IsNamespaced {
		return fmt.Sprintf("%v %v/%v", d.Namespace, d.Kind, d.Name)
	}
	return fmt.Sprintf("%v/%v", d.Kind, d.Name)
}

func newObjectDescription(uo unstructured.Unstructured, namespaced bool) ObjectDescription {
	uo.GetAPIVersion()
	return ObjectDescription{
		APIVersion:      uo.GetAPIVersion(),
		Kind:            uo.GetKind(),
		Name:            uo.GetName(),
		Namespace:       uo.GetNamespace(),
		UID:             uo.GetUID(),
		OwnerReferences: uo.GetOwnerReferences(),
		IsNamespaced:    namespaced,
	}
}

type ObjectCatalog map[types.UID]ObjectDescription

func main() {
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	config, err := ctrl.GetConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	cl, err := client.New(config, client.Options{})
	if err != nil {
		panic(err.Error())
	}

	// Load all the Kubernetes objects
	oc := ObjectCatalog{}
	numResources := 0
	resources, err := clientset.ServerPreferredResources()
	if err != nil {
		panic(err.Error())
	}
	for _, resourceList := range resources {
		// fmt.Printf("Loading %v... ", resourceList.GroupVersion)
		gv, err := schema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			panic(err.Error())
		}
		for _, r := range resourceList.APIResources {
			hasList := false
			for _, v := range r.Verbs {
				if v == "list" {
					hasList = true
				}
			}
			if hasList {
				ul := &unstructured.UnstructuredList{}
				ul.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   gv.Group,
					Version: gv.Version,
					Kind:    r.Kind,
				})
				err = cl.List(context.TODO(), ul)
				if err != nil {
					fmt.Printf("Error during list of %v: %v\n", ul.GroupVersionKind(), err)
				}
				for _, uo := range ul.Items {
					numResources++
					oc[uo.GetUID()] = newObjectDescription(uo, r.Namespaced)
				}
			}
		}
	}
	fmt.Printf("Discovered %v resources\n", numResources)

	// Validate owner references
	foundErrors := false
	checkedObj := 0
	checkedOwners := 0
	for _, obj := range oc {
		checkedObj++
		// fmt.Printf("Checking: %v\n", obj.KindNamespaceName())
		for _, ref := range obj.OwnerReferences {
			hasController := false
			owner, found := oc[ref.UID]
			if !found {
				// The owner doesn't exist, so nothing to check
				fmt.Printf("Warning: Couldn't find owner for %v/%v\n", obj.Namespace, obj.Name)
				continue
			}
			checkedOwners++
			if !hasController {
				hasController = true
			} else {
				foundErrors = true
				fmt.Printf("ERROR: Object %v has more than 1 controller\n", obj.KindNamespaceName())
			}
			// Check the rules
			if !obj.IsNamespaced && owner.IsNamespaced {
				foundErrors = true
				fmt.Printf("ERROR: Non-namespaced %v is owned by namespaced %v\n",
					obj.KindNamespaceName(), owner.KindNamespaceName())
			}
			if obj.IsNamespaced && owner.IsNamespaced && obj.Namespace != owner.Namespace {
				foundErrors = true
				fmt.Printf("ERROR: namespaced %v is owned by object in another namespace %v\n",
					obj.KindNamespaceName(), owner.KindNamespaceName())
			}
			if !strings.EqualFold(ref.Kind, owner.Kind) {
				foundErrors = true
				fmt.Printf("Warning: In object %v, owner ref kind (%v) does not match owner %v (%v).\n",
					obj.KindNamespaceName(), ref.Kind, owner.KindNamespaceName(), owner.Kind)
			}
			if !strings.EqualFold(ref.Name, owner.Name) {
				foundErrors = true
				fmt.Printf("Warning: In object %v, owner ref name (%v) does not match owner %v (%v).\n",
					obj.KindNamespaceName(), ref.Name, owner.KindNamespaceName(), owner.Name)
			}
			if !strings.EqualFold(ref.APIVersion, owner.APIVersion) {
				foundErrors = true
				fmt.Printf("Warning: In object %v, owner ref APIVersion (%v) does not match owner %v (%v).\n",
					obj.KindNamespaceName(), ref.APIVersion, owner.KindNamespaceName(), owner.APIVersion)
			}
		}
	}

	fmt.Printf("Scanned %v objects\n", checkedObj)
	fmt.Printf("Checked %v owner references\n", checkedOwners)

	if foundErrors {
		fmt.Printf("=== ERRORS FOUND ===\n")
		os.Exit(1)
	} else {
		fmt.Printf("All OK!\n")
	}
}

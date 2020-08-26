/*
Copyright 2020 Vladislav Poberezhny.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/prometheus/common/log"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	paasv1beta1 "operator/api/v1beta1"

	"operator/pkg/utils"
)

// ManagedResourceReconciler reconciles a ManagedResource object
type ManagedResourceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=paas.il,resources=managedresources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=paas.il,resources=managedresources/status,verbs=get;update;patch

func (r *ManagedResourceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("managedresource", req.NamespacedName)

	// Get managed resource k8s object
	managedResource := &paasv1beta1.ManagedResource{}
	if err := r.Get(ctx, req.NamespacedName, managedResource); err != nil {
		log.Error(err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Get managed resource bytes
	managedResourceBytes, err := utils.GetManagedResourceBytes(managedResource.Spec.Source)
	if err != nil {
		log.Error(err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Decode managed resource bytes to runtime object
	managedObject, _, err := utils.ObjectSerializer.Decode(managedResourceBytes, nil, &unstructured.Unstructured{})
	if err != nil {
		log.Error(err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Get managed resource object key
	managedObjectKey, err := client.ObjectKeyFromObject(managedObject)
	if err != nil {
		log.Error(err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Try getting object from cluster
	clusterObject := managedObject.DeepCopyObject()
	if err = r.Client.Get(ctx, managedObjectKey, clusterObject); err != nil {

		// Create the managed resource
		if err := r.Client.Create(ctx, managedObject); err != nil {
			log.Error(err)
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

	} else {

		// Insert .metadata.resourceVersion field into managed object
		if err := utils.CopyResourceVersion(clusterObject, &managedObject); err != nil {
			log.Error(err)
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		// Update the managed resource
		if err := r.Client.Update(ctx, managedObject); err != nil {
			log.Error(err)
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

	}

	// Deny management of a resource
	managedResource.Status.State = utils.StateEnabled
	managedResource.Status.Info = "Managing resource"
	if err := r.Status().Update(ctx, managedResource); err != nil {
		log.Error(err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return ctrl.Result{}, nil
}

func (r *ManagedResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&paasv1beta1.ManagedResource{}).
		Complete(r)
}

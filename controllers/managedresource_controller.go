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
	"errors"

	"github.com/go-logr/logr"
	"github.com/prometheus/common/log"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

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

// Reconcile reconciles a received resource
func (r *ManagedResourceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("managedresource", req.NamespacedName)

	// Get managed resource k8s object
	managedResource := &paasv1beta1.ManagedResource{}
	if err := r.Get(ctx, req.NamespacedName, managedResource); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err)
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Process object source
	_, _, managedObject, managedObjectKey, err := utils.ProcessSource(managedResource.Spec.Source)
	if err != nil {
		log.Error(err)
		return ctrl.Result{}, err
	}

	managedObjectFinalizer := "managedobject.finalizers.managedresources.paas.il"

	// Delete the managed resource if its CR is being deleted
	if !managedResource.DeletionTimestamp.IsZero() && controllerutil.ContainsFinalizer(managedResource, managedObjectFinalizer) {
		controllerutil.RemoveFinalizer(managedResource, managedObjectFinalizer)

		// Delete resource if it exists
		if err := r.Client.Delete(ctx, managedObject); err != nil && !apierrors.IsNotFound(err) {
			log.Error(err)
			return ctrl.Result{}, err
		}

		// Update finalizers field
		if err := r.Client.Update(ctx, managedResource); err != nil {
			log.Error(err)
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// Add finalizer for managed resource
	controllerutil.AddFinalizer(managedResource, managedObjectFinalizer)

	// Annotate managed resource with its owner namespace
	managedResourceAnnotations := managedObject.(controllerutil.Object).GetAnnotations()
	if managedResourceAnnotations == nil {
		managedResourceAnnotations = make(map[string]string)
	}
	managedResourceAnnotations[utils.ManagedResourceAnnotation] = req.NamespacedName.String()
	managedObject.(controllerutil.Object).SetAnnotations(managedResourceAnnotations)

	// Try getting object from cluster
	clusterObject := managedObject.DeepCopyObject()
	if err = r.Client.Get(ctx, managedObjectKey, clusterObject); err != nil {

		if apierrors.IsNotFound(err) {

			// Create the managed resource
			if err := r.Client.Create(ctx, managedObject); err != nil {
				log.Error(err)
				return ctrl.Result{}, errors.New("an error occurred while trying to create the object: " + err.Error())
			}

		} else {
			log.Error(err)
			return ctrl.Result{}, err
		}

	} else {

		// Insert .metadata.resourceVersion field into managed object
		managedObject.(controllerutil.Object).SetResourceVersion(clusterObject.(controllerutil.Object).GetResourceVersion())

		// Update the managed resource
		if err := r.Client.Update(ctx, managedObject); err != nil {
			log.Error(err)
			return ctrl.Result{}, errors.New("an error occurred while trying to update the object: " + err.Error())
		}
	}

	// Update managed resource with finalizer field
	if err := r.Update(context.Background(), managedResource); err != nil {
		log.Error(err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager registers controller with the manager
func (r *ManagedResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&paasv1beta1.ManagedResource{}).
		Complete(r)
}

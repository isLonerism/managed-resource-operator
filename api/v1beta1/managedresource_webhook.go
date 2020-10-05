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

package v1beta1

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/fatih/structs"
	"github.com/jeremywohl/flatten"
	"sigs.k8s.io/yaml"

	"operator/pkg/utils"
)

// log is for logging in this package.
var managedresourcelog = logf.Log.WithName("managedresource-resource")

// SetupWebhookWithManager registers webhooks with the controller manager
func (r *ManagedResource) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-paas-il-v1beta1-managedresource,mutating=true,failurePolicy=fail,groups=paas.il,resources=managedresources,verbs=create;update,versions=v1beta1,name=mmanagedresource.kb.io
// +kubebuilder:rbac:groups=paas.il,resources=managedresourcebindings,verbs=get;list;watch

var _ webhook.Defaulter = &ManagedResource{}

func getClient() (client.Client, error) {

	// Add new resources to scheme
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		return nil, err
	}

	// Init client
	k8sClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, err
	}

	return k8sClient, nil
}

func checkPermissions(r *utils.ManagedResourceStruct, crNamespace utils.Namespace) (bool, error) {

	// Get flat map from target struct
	targetMap, err := flatten.Flatten(structs.Map(r), "", flatten.DotStyle)
	if err != nil {
		return false, err
	}

	// Get updated k8s client
	k8sClient, err := getClient()
	if err != nil {
		return false, err
	}

	// List all bindings
	bindings := &ManagedResourceBindingList{}
	if err := k8sClient.List(context.Background(), bindings, &client.ListOptions{}); err != nil {
		return false, err
	}

	for _, binding := range bindings.Items {

		// Check if namespace is present
		for _, namespace := range binding.Spec.Namespaces {
			if namespace == "*" || namespace == crNamespace {

				// Check if object is present
				for _, object := range binding.Spec.Objects {

					// Get flat map from object struct
					objectMap, err := flatten.Flatten(structs.Map(object), "", flatten.DotStyle)
					if err != nil {
						return false, err
					}

					// Find matching object
					match := true
					for key, value := range objectMap {
						if value != "*" && value != targetMap[key] {
							match = false
							break
						}
					}

					// Allow creation if match found
					if match {
						return true, nil
					}

				}

				break
			}
		}
	}

	return false, nil
}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *ManagedResource) Default() {
	managedresourcelog.Info("default", "name", r.Name)

	// Get managed resource bytes
	managedResourceBytes, _, _, _, err := utils.ProcessSource(r.Spec.Source)
	if err != nil {
		return
	}

	// Convert YAML bytes to JSON bytes for raw extension
	managedResourceBytesJSON, err := yaml.YAMLToJSON(managedResourceBytes)
	if err != nil {
		return
	}

	// New raw object struct with managed resource bytes
	objectSource := runtime.RawExtension{
		Raw: managedResourceBytesJSON,
	}

	// New source struct with only the Object field defined
	r.Spec.Source = utils.SourceStruct{
		Object: objectSource,
	}

	// Set object state as 'pending'
	r.Status.State = utils.StatePending
	r.Status.Info = "Object is pending for creation"
}

// +kubebuilder:webhook:verbs=create;update;delete,path=/validate-paas-il-v1beta1-managedresource,mutating=false,failurePolicy=fail,groups=paas.il,resources=managedresources,versions=v1beta1,name=vmanagedresource.kb.io

var _ webhook.Validator = &ManagedResource{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ManagedResource) ValidateCreate() error {
	managedresourcelog.Info("validate create", "name", r.Name)

	// -- Ensure a single source exists --

	// Process object source
	_, newManagedResourceStruct, newManagedObject, newManagedObjectKey, err := utils.ProcessSource(r.Spec.Source)
	if err != nil {
		return err
	}

	// -- Check permissions ---

	// Check for permission
	allowed, err := checkPermissions(newManagedResourceStruct, utils.Namespace(r.Namespace))
	if err != nil {
		return err
	} else if !allowed {
		return errors.New("permission denied")
	}

	// -- Ensure object does not already exist --

	// Get updated k8s client
	k8sClient, err := getClient()
	if err != nil {
		return err
	}

	// Get managed resource key
	newManagedResourceKey, err := client.ObjectKeyFromObject(r)
	if err != nil {
		return err
	}

	// Try getting object from cluster
	clusterObject := newManagedObject.DeepCopyObject()
	if err := k8sClient.Get(context.Background(), newManagedObjectKey, clusterObject); err != nil {

		if !apierrors.IsNotFound(err) {
			return err
		}

		// If exists - check if managed by the current managed resource CR
	} else if clusterObject.(controllerutil.Object).GetAnnotations() == nil ||
		clusterObject.(controllerutil.Object).GetAnnotations()[utils.ManagedResourceAnnotation] != newManagedResourceKey.String() {
		return errors.New("object already exists")
	}

	// -- Ensure there are no other errors during creation --

	// Try dry-run creation
	if err := k8sClient.Create(context.Background(), newManagedObject, &client.CreateOptions{
		DryRun: []string{"All"},
	}); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ManagedResource) ValidateUpdate(old runtime.Object) error {
	managedresourcelog.Info("validate update", "name", r.Name)

	if err := r.ValidateCreate(); err != nil {
		return err
	}

	// Old CR bytes
	var oldManagedResourceBytesBuffer bytes.Buffer
	if err := utils.ObjectSerializer.Encode(old, io.Writer(&oldManagedResourceBytesBuffer)); err != nil {
		return err
	}

	// Old CR
	oldManagedResource := &ManagedResource{}
	if err := yaml.Unmarshal(oldManagedResourceBytesBuffer.Bytes(), oldManagedResource); err != nil {
		return err
	}

	// Process both objects
	_, oldManagedResourceStruct, _, _, err := utils.ProcessSource(oldManagedResource.Spec.Source)
	if err != nil {
		return err
	}
	_, newManagedResourceStruct, _, _, err := utils.ProcessSource(r.Spec.Source)
	if err != nil {
		return err
	}

	// Ensure that the structs are equal
	if !reflect.DeepEqual(newManagedResourceStruct, oldManagedResourceStruct) {
		return errors.New("new managed resource must manage the same object as the old managed resource")
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ManagedResource) ValidateDelete() error {
	managedresourcelog.Info("validate delete", "name", r.Name)

	// Process given object
	_, _, managedObject, _, err := utils.ProcessSource(r.Spec.Source)
	if err != nil {
		return err
	}

	// Get new k8s client
	k8sClient, err := getClient()
	if err != nil {
		return err
	}

	// Try dry-run deletion of managed object
	if err := k8sClient.Delete(context.Background(), managedObject, &client.DeleteOptions{
		DryRun: []string{"All"},
	}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

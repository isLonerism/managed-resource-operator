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
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/fatih/structs"
	"github.com/jeremywohl/flatten"
	"sigs.k8s.io/yaml"

	"operator/pkg/utils"
)

// log is for logging in this package.
var managedresourcelog = logf.Log.WithName("managedresource-resource")

// k8sClient is for querying API for dry-runs and bindings
var k8sClient client.Client = nil

// SetupWebhookWithManager registers webhooks with the controller manager
func (r *ManagedResource) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-paas-il-v1beta1-managedresource,mutating=true,failurePolicy=fail,groups=paas.il,resources=managedresources,verbs=create;update,versions=v1beta1,name=mmanagedresource.kb.io
// +kubebuilder:rbac:groups=paas.il,resources=managedresourcebindings,verbs=get;list;watch

var _ webhook.Defaulter = &ManagedResource{}

func getClient() client.Client {

	if k8sClient == nil {

		// Add new resources to scheme
		scheme := runtime.NewScheme()
		if err := AddToScheme(scheme); err != nil {
			panic(err)
		}

		// Init kubernetes client
		k8sClient, _ = client.New(ctrl.GetConfigOrDie(), client.Options{
			Scheme: scheme,
		})
	}

	return k8sClient
}

func checkPermissions(r *utils.ManagedResourceStruct, crNamespace utils.Namespace, verb utils.Verb) error {

	// 'contains' function for string slices
	contains := func(list interface{}, match interface{}) bool {
		slice := reflect.ValueOf(list)

		for i := 0; i < slice.Len(); i++ {
			if slice.Index(i).String() == reflect.ValueOf(match).String() {
				return true
			}
		}

		return false
	}

	// Get flat map from target struct
	targetMap, err := flatten.Flatten(structs.Map(r), "", flatten.DotStyle)
	if err != nil {
		return err
	}

	// List all bindings
	bindings := &ManagedResourceBindingList{}
	if err := getClient().List(context.Background(), bindings, &client.ListOptions{}); err != nil {
		return err
	}

	// Iterate bindings
	for _, binding := range bindings.Items {

		if contains(binding.Spec.Namespaces, crNamespace) || contains(binding.Spec.Namespaces, "*") {

			// Check if object is present
			for _, item := range binding.Spec.Items {

				// Get flat map from object struct
				objectMap, err := flatten.Flatten(structs.Map(item.Object), "", flatten.DotStyle)
				if err != nil {
					return err
				}

				// Find matching object
				match := true
				for key, value := range objectMap {
					if value != "*" && value != targetMap[key] {
						match = false
						break
					}
				}

				// Allow if match and verb are found
				if match && contains(item.Verbs, verb) {
					return nil
				}
			}
		}
	}

	return errors.New("permission denied")
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

	// -- Check permissions --

	// Check for creation permission
	if err := checkPermissions(newManagedResourceStruct, utils.Namespace(r.Namespace), utils.VerbCreate); err != nil {
		return err
	}

	// -- Ensure object does not already exist --

	// Try getting object from cluster
	clusterObject := newManagedObject.DeepCopyObject()
	if err := getClient().Get(context.Background(), newManagedObjectKey, clusterObject); err != nil {

		if !apierrors.IsNotFound(err) {
			return err
		}

	} else {
		return errors.New("object already exists")
	}

	// -- Ensure there are no other errors during creation --

	// Try dry-run creation
	if err := getClient().Create(context.Background(), newManagedObject, &client.CreateOptions{
		DryRun: []string{"All"},
	}); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ManagedResource) ValidateUpdate(old runtime.Object) error {
	managedresourcelog.Info("validate update", "name", r.Name)

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
	_, newManagedResourceStruct, newManagedObject, _, err := utils.ProcessSource(r.Spec.Source)
	if err != nil {
		return err
	}

	// Check update permissions
	if err := checkPermissions(oldManagedResourceStruct, utils.Namespace(r.Namespace), utils.VerbUpdate); err != nil {
		return err
	}

	// Ensure that the structs are equal
	if !reflect.DeepEqual(newManagedResourceStruct, oldManagedResourceStruct) {
		return errors.New("new managed resource must manage the same object as the old managed resource")
	}

	// -- Ensure there are no other errors during update --

	// Try dry-run update
	if err := getClient().Update(context.Background(), newManagedObject, &client.UpdateOptions{
		DryRun: []string{"All"},
	}); err != nil {
		return err
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ManagedResource) ValidateDelete() error {
	managedresourcelog.Info("validate delete", "name", r.Name)

	// Process given object
	_, managedResourceStruct, managedObject, _, err := utils.ProcessSource(r.Spec.Source)
	if err != nil {
		return err
	}

	// Check deletion permissions
	if err := checkPermissions(managedResourceStruct, utils.Namespace(r.Namespace), utils.VerbDelete); err != nil {
		return err
	}

	// -- Ensure there are no other errors during deletion --

	// Try dry-run deletion
	if err := getClient().Delete(context.Background(), managedObject, &client.DeleteOptions{
		DryRun: []string{"All"},
	}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

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
	"errors"
	"io"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"gopkg.in/yaml.v2"

	"operator/pkg/utils"
)

// log is for logging in this package.
var managedresourcelog = logf.Log.WithName("managedresource-resource")

func (r *ManagedResource) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-paas-il-v1beta1-managedresource,mutating=true,failurePolicy=fail,groups=paas.il,resources=managedresources,verbs=create;update,versions=v1beta1,name=mmanagedresource.kb.io

var _ webhook.Defaulter = &ManagedResource{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *ManagedResource) Default() {
	managedresourcelog.Info("default", "name", r.Name)

	// Get managed resource bytes
	managedResourceBytes, err := utils.GetManagedResourceBytes(r.Spec.Source)
	if err != nil || (err == nil && managedResourceBytes == nil) {
		return
	}

	// New source struct with only the YAML field defined
	r.Spec.Source = utils.SourceStruct{
		YAML: string(managedResourceBytes),
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:verbs=create;update,path=/validate-paas-il-v1beta1-managedresource,mutating=false,failurePolicy=fail,groups=paas.il,resources=managedresources,versions=v1beta1,name=vmanagedresource.kb.io

var _ webhook.Validator = &ManagedResource{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ManagedResource) ValidateCreate() error {
	managedresourcelog.Info("validate create", "name", r.Name)

	// Ensure a single source exists
	newManagedResourceBytes, err := utils.GetManagedResourceBytes(r.Spec.Source)
	if err != nil {
		return err
	} else if newManagedResourceBytes == nil {
		return errors.New("a single source must be defined")
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

	// Get both objects as bytes
	oldManagedResourceBytes, err := utils.GetManagedResourceBytes(oldManagedResource.Spec.Source)
	if err != nil {
		return err
	}
	newManagedResourceBytes, err := utils.GetManagedResourceBytes(r.Spec.Source)
	if err != nil {
		return err
	}

	// Unmarshal both objects to structs
	oldManagedResourceStruct := &utils.ManagedResourceStruct{}
	if err := yaml.Unmarshal(oldManagedResourceBytes, oldManagedResourceStruct); err != nil {
		return err
	}
	newManagedResourceStruct := &utils.ManagedResourceStruct{}
	if err := yaml.Unmarshal(newManagedResourceBytes, newManagedResourceStruct); err != nil {
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

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

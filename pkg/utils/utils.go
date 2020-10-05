package utils

import (
	"errors"
	"io/ioutil"
	"net/http"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kubeyaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// ManagedState is a custom state type for a managed resource
type ManagedState string

// Valid states for a managed resource
const (
	StateManaged = "Managed"
	StatePending = "Pending"
	StateError   = "Error"
)

// ObjectSerializer is a runtime object/byte stream codec
var ObjectSerializer = kubeyaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

// ManagedResourceAnnotation is a reference to the objects owner CR
var ManagedResourceAnnotation = "managedresources.paas.il/owner"

// Namespace is an alias for a namespace string
// +kubebuilder:validation:MaxLength=63
// +kubebuilder:validation:Pattern="(^[a-z0-9]([-a-z0-9]*[a-z0-9])?$)|(^[*]$)"
type Namespace string

// MetadataStruct is a stripped metadata object
type MetadataStruct struct {

	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern="(^[a-z0-9]([-a-z0-9]*[a-z0-9])?([.][a-z0-9]([-a-z0-9]*[a-z0-9])?)*$)|(^[*]$)"
	Name string `json:"name"`

	Namespace Namespace `json:"namespace,omitempty"`
}

// ManagedResourceStruct is a reference to an object to be managed
type ManagedResourceStruct struct {

	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern="(^[a-zA-Z]([-a-zA-Z0-9]*[a-zA-Z0-9])?$)|(^[*]$)"
	Kind string `json:"kind"`

	Metadata MetadataStruct `json:"metadata"`
}

// SourceStruct defines options to supply the managed object code
type SourceStruct struct {
	URL  string `json:"url,omitempty"`
	YAML string `json:"yaml,omitempty"`

	// +kubebuilder:validation:XEmbeddedResource
	// +kubebuilder:validation:XPreserveUnknownFields
	// +nullable
	Object runtime.RawExtension `json:"object,omitempty"`
}

// DeepCopyInto is a custom deep copy method for source struct which controller-gen expects
func (r *SourceStruct) DeepCopyInto(out *SourceStruct) {
	(*out).URL = r.URL
	(*out).YAML = r.YAML
	r.Object.DeepCopyInto(&out.Object)
}

// A map of source types and their appropriate retrieval methods
var sourceFunctions = map[string]func(SourceStruct) ([]byte, error){
	"URL":    getManagedResourceBytesByURL,
	"YAML":   getManagedResourceBytesByYAML,
	"Object": getManagedResourceBytesByObject,
}

func getManagedResourceBytes(sourceStruct SourceStruct) ([]byte, error) {

	// Init resource bytes
	var managedResourceBytes []byte

	// Store all names and values of source types
	sourceNames := reflect.TypeOf(sourceStruct)
	sourceValues := reflect.ValueOf(sourceStruct)

	// Iterate existing source types
	for sourceIndex := 0; sourceIndex < sourceNames.NumField() && managedResourceBytes == nil; sourceIndex++ {
		sourceName := sourceNames.Field(sourceIndex)
		sourceValue := sourceValues.Field(sourceIndex)

		// Find the defined source type and call the appropriate method
		if sourceValue.String() != "" {
			managedResourceBytes, err := sourceFunctions[sourceName.Name](sourceStruct)
			if err != nil {
				return nil, err
			}
			return managedResourceBytes, nil
		}
	}

	return nil, nil
}

func getManagedResourceBytesByURL(sourceStruct SourceStruct) ([]byte, error) {

	// HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Get resource yaml from remote
	response, err := client.Get(sourceStruct.URL)
	if err != nil {
		return nil, errors.New("an error occurred while querying " + sourceStruct.URL + ": " + err.Error())
	} else if response.StatusCode != 200 {
		return nil, errors.New("an error occurred while querying " + sourceStruct.URL + ": " + response.Status)
	}
	defer response.Body.Close()

	// Read response as byte array
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, errors.New("an error occurred while reading response from " + sourceStruct.URL + ": " + err.Error())
	}

	return body, nil
}

func getManagedResourceBytesByYAML(sourceStruct SourceStruct) ([]byte, error) {
	return []byte(sourceStruct.YAML), nil
}

func getManagedResourceBytesByObject(sourceStruct SourceStruct) ([]byte, error) {

	// Write raw json bytes as yaml bytes
	embeddedYAMLBytes, err := yaml.JSONToYAML(sourceStruct.Object.Raw)
	if err != nil {
		return nil, errors.New("an error occurred while reading object: " + err.Error())
	}

	return embeddedYAMLBytes, nil
}

// ProcessSource reads ManagedObject source struct and returns its relevant formats
func ProcessSource(source SourceStruct) ([]byte, *ManagedResourceStruct, runtime.Object, types.NamespacedName, error) {

	// Get managed resource bytes
	managedResourceBytes, err := getManagedResourceBytes(source)
	if err != nil {
		return nil, nil, nil, types.NamespacedName{}, errors.New("an error occurred while trying to read the source: " + err.Error())
	} else if managedResourceBytes == nil {
		return nil, nil, nil, types.NamespacedName{}, errors.New("an error occurred while trying to read the source: a single source must be defined")
	}

	// Unmarshal bytes to managed resource struct
	managedResourceStruct := &ManagedResourceStruct{}
	if err := yaml.Unmarshal(managedResourceBytes, managedResourceStruct); err != nil {
		return nil, nil, nil, types.NamespacedName{}, errors.New("an error occured while trying to read object data: " + err.Error())
	}

	// Decode managed resource bytes to runtime object
	managedObject, _, err := ObjectSerializer.Decode(managedResourceBytes, nil, &unstructured.Unstructured{})
	if err != nil {
		return nil, nil, nil, types.NamespacedName{}, errors.New("an error occurred while trying to unmarshal object yaml: " + err.Error())
	}

	// Get managed object key
	managedObjectKey, err := client.ObjectKeyFromObject(managedObject)
	if err != nil {
		return nil, nil, nil, types.NamespacedName{}, errors.New("an error occurred while trying to get object key: " + err.Error())
	}

	return managedResourceBytes, managedResourceStruct, managedObject, managedObjectKey, nil
}

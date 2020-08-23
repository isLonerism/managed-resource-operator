package utils

import (
	"io/ioutil"
	"net/http"
	"reflect"

	"gopkg.in/yaml.v2"
)

// ManagedState is a custom state type for a managed resource
type ManagedState string

// Valid states for a managed resource
const (
	StateEnabled  = "Enabled"
	StateDisabled = "Disabled"
	StateDenied   = "Denied"
	StateError    = "Error"
)

// ManagedResourceStruct is a reference to an object to be managed
type ManagedResourceStruct struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace,omitempty"`
}

// SourceStruct defines options to supply the managed object code
type SourceStruct struct {
	URL  string `json:"url,omitempty"`
	Lala string `json:"lala,omitempty"`
}

// ResourceMeta holds the name and namespace fields on a k8s object
type ResourceMeta struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace,omitempty"`
}

// Resource holds basic fields of any k8s object
type Resource struct {
	APIVersion string       `yaml:"apiVersion"`
	Kind       string       `yaml:"kind"`
	Metadata   ResourceMeta `yaml:"metadata"`
}

// A map of source types and their appropriate retrieval methods
var sourceFunctions = map[string]func(SourceStruct) ([]byte, error){
	"URL": getManagedResourceBytesByURL,
}

// GetManagedResourceBytes retrieves the managed object yaml as byte array
func GetManagedResourceBytes(sourceStruct SourceStruct) ([]byte, error) {

	// Store all names and values of source types
	sourceNames := reflect.TypeOf(sourceStruct)
	sourceValues := reflect.ValueOf(sourceStruct)

	// Iterate existing source types
	for sourceIndex := 0; sourceIndex < sourceNames.NumField(); sourceIndex++ {
		sourceName := sourceNames.Field(sourceIndex)
		sourceValue := sourceValues.Field(sourceIndex)

		// Find the defined source type and call the appropriate method
		if sourceValue.String() != "" {
			return sourceFunctions[sourceName.Name](sourceStruct)
		}
	}

	return nil, nil
}

func getManagedResourceBytesByURL(sourceStruct SourceStruct) ([]byte, error) {

	// Get resource yaml from remote
	response, err := http.Get(sourceStruct.URL)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	// Read response as byte array
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

// GetManagedResourceMapFromBytes unmarshals resource bytes to a map
func GetManagedResourceMapFromBytes(managedResourceBytes []byte) (map[string]interface{}, error) {

	// Unmarshal the resource to a map
	managedResource := map[string]interface{}{}
	if err := yaml.Unmarshal(managedResourceBytes, &managedResource); err != nil {
		return nil, err
	}

	return managedResource, nil
}

package utils

import (
	"io/ioutil"
	"net/http"
	"reflect"
)

// ManagedState is a custom state type for a managed resource
type ManagedState string

// Valid states for a managed resource
const (
	StateEnabled  = "Enabled"
	StateDisabled = "Disabled"
	StatePending  = "Pending"
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
	URL string `json:"url,omitempty"`
}

// A map of source types and their appropriate retrieval methods
var sourceFunctions = map[string]func(SourceStruct) ([]byte, error){
	"URL": getManagedResourceBytesByURL,
}

// GetManagedResourceBytes returns the managed object yaml as bytes
func GetManagedResourceBytes(sourceStruct SourceStruct) ([]byte, error) {

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

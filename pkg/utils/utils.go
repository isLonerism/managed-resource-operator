package utils

import (
	"errors"
	"io/ioutil"
	"net/http"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kubeyaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
)

// ManagedState is a custom state type for a managed resource
type ManagedState string

// Valid states for a managed resource
const (
	StateManaged    = "Managed"
	StateNotManaged = "Not Managed"
	StatePending    = "Pending"
	StateError      = "Error"
)

// ObjectSerializer is a runtime object/byte stream codec
var ObjectSerializer = kubeyaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

// MetadataStruct is a stripped metadata object
type MetadataStruct struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// ManagedResourceStruct is a reference to an object to be managed
type ManagedResourceStruct struct {
	Kind     string         `json:"kind"`
	Metadata MetadataStruct `json:"metadata"`
}

// SourceStruct defines options to supply the managed object code
type SourceStruct struct {
	URL  string `json:"url,omitempty"`
	YAML string `json:"yaml,omitempty"`
}

// ResourceOwnerAnnotation is an identifier of resource manager
var ResourceOwnerAnnotation = "managedresources.paas.il/owner"

// A map of source types and their appropriate retrieval methods
var sourceFunctions = map[string]func(SourceStruct) ([]byte, error){
	"URL":  getManagedResourceBytesByURL,
	"YAML": getManagedResourceBytesByYAML,
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

	// HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Get resource yaml from remote
	response, err := client.Get(sourceStruct.URL)
	if err != nil {
		return nil, errors.New("an error occured while querying " + sourceStruct.URL + ": " + err.Error())
	} else if response.StatusCode != 200 {
		return nil, errors.New("an error occured while querying " + sourceStruct.URL + ": " + response.Status)
	}
	defer response.Body.Close()

	// Read response as byte array
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, errors.New("an error occured while reading response from " + sourceStruct.URL + ": " + err.Error())
	}

	return body, nil
}

func getManagedResourceBytesByYAML(sourceStruct SourceStruct) ([]byte, error) {
	return []byte(sourceStruct.YAML), nil
}

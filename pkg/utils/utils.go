package utils

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/imdario/mergo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kubeyaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// Valid verbs for managed resource bindings
const (
	VerbCreate = "create"
	VerbDelete = "delete"
)

// ObjectSerializer is a runtime object/byte stream codec
var ObjectSerializer = kubeyaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

// ManagedResourceAnnotation is a reference to the objects owner CR
var ManagedResourceAnnotation = "managedresources.paas.il/owner"

// Namespace is an alias for a namespace string
// +kubebuilder:validation:MaxLength=63
// +kubebuilder:validation:Pattern="(^[a-z0-9]([-a-z0-9]*[a-z0-9])?$)|(^[*]$)"
type Namespace string

// Verb is an alias for a permission verb string
// +kubebuilder:validation:Enum=create;delete
type Verb string

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

	// Parse HTTP_INSECURE environment variable
	HTTP_INSECURE_STR, ok := os.LookupEnv("HTTP_INSECURE")
	if !ok {
		HTTP_INSECURE_STR = "false"
	}
	HTTP_INSECURE, err := strconv.ParseBool(HTTP_INSECURE_STR)
	if err != nil {
		return nil, errors.New("an error occurred while parsing HTTP_INSECURE environment variable: " + err.Error())
	}

	// Parse HTTP_TIMEOUT environment variable
	HTTP_TIMEOUT_STR, ok := os.LookupEnv("HTTP_TIMEOUT")
	if !ok {
		HTTP_TIMEOUT_STR = "10"
	}
	HTTP_TIMEOUT, err := strconv.ParseInt(HTTP_TIMEOUT_STR, 10, 64)
	if err != nil {
		return nil, errors.New("an error occurred while parsing HTTP_TIMEOUT environment variable: " + err.Error())
	}

	// Define new TLS config
	tlsConfig := &tls.Config{
		InsecureSkipVerify: HTTP_INSECURE,
		RootCAs:            x509.NewCertPool(),
	}

	// Read and use the CA bundle if present
	HTTP_CA_BUNDLE_PATH, ok := os.LookupEnv("HTTP_CA_BUNDLE_PATH")
	if ok {

		// Read CA bundle
		CABundle, err := ioutil.ReadFile(HTTP_CA_BUNDLE_PATH)
		if err != nil {
			return nil, errors.New("an error occurred while reading CA bundle: " + err.Error())
		}

		// Add bundle to trusted CAs
		tlsConfig.RootCAs.AppendCertsFromPEM(CABundle)
	}

	// HTTP client with timeout
	client := &http.Client{
		Timeout: time.Duration(HTTP_TIMEOUT) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
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
		return nil, nil, nil, types.NamespacedName{}, errors.New("an error occurred while trying to read object data: " + err.Error())
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

// ProcessOverwrite merges resource and overwrite fields
func ProcessOverwrite(managedResourceBytes []byte, overwrite runtime.RawExtension) ([]byte, error) {

	// Do not overwrite if there is nothing to overwrite with
	if overwrite.Raw == nil {
		return managedResourceBytes, nil
	}

	// Define empty maps for resource and overwrite
	var managedResourceMap map[string]interface{}
	var overwriteMap map[string]interface{}

	// Unmarshal both objects to their maps
	if err := yaml.Unmarshal(managedResourceBytes, &managedResourceMap); err != nil {
		return nil, errors.New("an error occurred while trying to unmarshal resource: " + err.Error())
	}
	if err := json.Unmarshal(overwrite.Raw, &overwriteMap); err != nil {
		return nil, errors.New("an error occurred while trying to unmarshal overwrite: " + err.Error())
	}

	// Union merge maps with overwrite
	if err := mergo.MergeWithOverwrite(&managedResourceMap, overwriteMap); err != nil {
		return nil, errors.New("an error occurred while trying to overwrite parameters: " + err.Error())
	}

	// Marshal final resource as bytes
	managedResourceBytesOverwrite, err := yaml.Marshal(managedResourceMap)
	if err != nil {
		return nil, errors.New("an error occurred while trying to marshal overwritten resource: " + err.Error())
	}

	return managedResourceBytesOverwrite, nil
}

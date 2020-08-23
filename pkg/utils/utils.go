package utils

// Reference to an object to be managed
type ManagedResourceStruct struct {
	APIGroup  string `json:"apiGroup"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

package types

// import (
// 	"k8s.io/apimachinery/pkg/runtime/schema"
// )

const (
	// AppName represents the full app name
	AppName string = "move2kube"
	// AppNameShort represents the short app name
	AppNameShort string = "m2k"
	// GroupName is the group name use in this package
	GroupName = AppName + ".konveyor.io"
)

// TypeMeta stores apiversion and kind for resources
type TypeMeta struct {
	// APIVersion defines the versioned schema of this representation of an object.
	APIVersion string `yaml:"apiVersion,omitempty" json:"apiVersion,omitempty"`
	// Kind is a string value representing the resource this object represents.
	Kind string `yaml:"kind,omitempty" json:"kind,omitempty"`
}

// ObjectMeta stores object metadata
type ObjectMeta struct {
	// Name represents the name of the resource
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	// Labels are Map of string keys and values that can be used to organize and categorize (scope and select) objects.
	Labels map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// Kind stores the kind of the file
type Kind string

// var (
// 	// SchemeGroupVersion is group version used to register these objects
// 	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1alpha1"}
// )

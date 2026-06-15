package channel

import "fmt"

type UnsupportedFeatureError struct {
	Provider string
	Feature  string
}

func NewUnsupportedFeatureError(provider, feature string) error {
	return &UnsupportedFeatureError{
		Provider: provider,
		Feature:  feature,
	}
}

func (e *UnsupportedFeatureError) Error() string {
	return fmt.Sprintf("%s adaptor: %s is not implemented", e.Provider, e.Feature)
}

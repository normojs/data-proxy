package secretref

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

const (
	SchemeEnv      = "env"
	ConfiguredMask = "configured"
)

var envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Reference struct {
	Scheme string
	Name   string
}

func (ref Reference) String() string {
	if ref.Scheme == "" || ref.Name == "" {
		return ""
	}
	return ref.Scheme + ":" + ref.Name
}

func Parse(raw string) (Reference, error) {
	raw = strings.TrimSpace(raw)
	scheme, name, ok := strings.Cut(raw, ":")
	if !ok || strings.ToLower(strings.TrimSpace(scheme)) != SchemeEnv {
		return Reference{}, errors.New("auth_ref must use env:NAME secret reference")
	}
	name = strings.TrimSpace(name)
	if !envNamePattern.MatchString(name) {
		return Reference{}, errors.New("auth_ref must use env:NAME secret reference")
	}
	return Reference{Scheme: SchemeEnv, Name: name}, nil
}

func Normalize(raw string) (string, error) {
	ref, err := Parse(raw)
	if err != nil {
		return "", err
	}
	return ref.String(), nil
}

func Validate(raw string) error {
	_, err := Parse(raw)
	return err
}

func ResolveEnv(raw string, label string) (string, error) {
	ref, err := Parse(raw)
	if err != nil {
		return "", err
	}
	value, ok := os.LookupEnv(ref.Name)
	if !ok || strings.TrimSpace(value) == "" {
		label = strings.TrimSpace(label)
		if label == "" {
			label = "auth"
		}
		return "", fmt.Errorf("%s secret reference is not configured", label)
	}
	return value, nil
}

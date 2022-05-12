package clusterconf

import (
	"errors"
	"fmt"
)

type Workload struct {
	Version    string           `yaml:"version"`
	ConfigType string           `yaml:"configType"`
	Metadata   WorkloadMetadata `yaml:"metadata"`
	Spec       WorkloadSpec     `yaml:"spec"`
}

func (w Workload) Validate() error {
	if w.Name() == "" {
		return fmt.Errorf("workload name must not be blank")
	}

	for _, exclusion := range w.Spec.Exclusions {
		if err := exclusion.validate(); err != nil {
			return err
		}
	}
	return nil
}

func (w Workload) Name() string {
	return w.Metadata.Name
}

type WorkloadMetadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type WorkloadSpec struct {
	Exclusions []Exclusion `yaml:"exclusions"`
}

type Operator string

const (
	OperatorNotEqual Operator = "NotEqual"
	OperatorEqual    Operator = "Equal"
)

func (o Operator) validate() error {
	switch o {
	case OperatorEqual, OperatorNotEqual:
		return nil
	}
	return fmt.Errorf("unknown operator: %s", o)
}

type Exclusion struct {
	Key      string   `yaml:"key"`
	Operator Operator `yaml:"operator"`
	Value    string   `yaml:"value"`
}

func (e Exclusion) validate() error {
	if e.Key == "" {
		return errors.New("Exclusion.Key must not be empty")
	}
	if e.Value == "" {
		return errors.New("Exclusion.Value must not be empty")
	}
	return e.Operator.validate()
}

func (e Exclusion) Excludes(labels Labels) bool {
	for key, value := range labels {
		switch e.Operator {
		case OperatorNotEqual:
			if e.Key == key && e.Value != value {
				return true

			}
		case OperatorEqual:
			if e.Key == key && e.Value == value {
				return true

			}
		}
	}
	return false
}

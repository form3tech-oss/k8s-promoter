package environment

import (
	"fmt"
)

const (
	None Env = "no environment"

	// SourceManifest not an environment, but it's a source of manifests.
	SourceManifest Env = "manifests"

	Development Env = "development"
	Test        Env = "test"
	Production  Env = "production"
)

var ErrUnknownEnvironment = fmt.Errorf("unknown environment")

type Env string

func (e Env) Validate() error {
	if e != Development && e != Test && e != Production {
		return fmt.Errorf("env '%s' is not one of %s, %s, %s", e, Development, Test, Production)
	}
	return nil
}

func (e Env) ManifestSource() (Env, error) {
	switch e {
	case Production:
		return Test, nil
	case Test:
		return Development, nil
	case Development:
		return SourceManifest, nil
	default:
		return "", fmt.Errorf("%s: %w", e, ErrUnknownEnvironment)
	}
}

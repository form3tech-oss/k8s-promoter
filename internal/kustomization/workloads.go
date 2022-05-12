package kustomization

import (
	"fmt"
	"path/filepath"
	"sort"
	"text/template"

	"github.com/form3tech/k8s-promoter/internal/clusterconf"
	"github.com/form3tech/k8s-promoter/internal/filesystem"
	"github.com/go-git/go-billy/v5"
	"github.com/sirupsen/logrus"
)

var kustomizationTemplate = `---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:{{range $element := .}}
  - ./{{$element}}{{end}}
`

const (
	KustomizationFile = "kustomization.yaml"
)

type Kust struct {
	logger *logrus.Entry
}

func NewKust(log *logrus.Entry) *Kust {
	return &Kust{
		logger: log,
	}
}

func (k *Kust) Write(fs billy.Filesystem, cluster clusterconf.Cluster) error {
	dirNames, err := filesystem.DirsInDir(fs, cluster.ManifestFolder())
	if err != nil {
		return fmt.Errorf("list workload directories: %w", err)
	}
	sort.Strings(dirNames)

	kustomizationPath := filepath.Join(cluster.ManifestFolder(), KustomizationFile)
	if len(dirNames) > 0 {
		return k.write(fs, kustomizationPath, dirNames)
	} else {
		return k.delete(fs, kustomizationPath)
	}
}

func (k *Kust) write(fs billy.Filesystem, kustomizationPath string, dirNames []string) error {
	k.logger.Debugf("Updating kustomization.yaml at '%s'", kustomizationPath)

	f, err := fs.Create(kustomizationPath)
	if err != nil {
		return fmt.Errorf("open kustomization.yaml: %w", err)
	}

	t, err := template.New("kustomization").Parse(kustomizationTemplate)
	if err != nil {
		return fmt.Errorf("parse kustomization template: %w", err)
	}

	if err = t.Execute(f, dirNames); err != nil {
		return fmt.Errorf("t.execute: %w", err)
	}

	if err = f.Close(); err != nil {
		return fmt.Errorf("f.Close: %w", err)
	}

	return nil
}

func (k *Kust) delete(fs billy.Filesystem, kustomizationPath string) error {
	k.logger.Debugf("Deleting kustomization.yaml at '%s'", kustomizationPath)
	return fs.Remove(kustomizationPath)
}

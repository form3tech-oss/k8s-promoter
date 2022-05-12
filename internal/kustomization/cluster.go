package kustomization

import (
	"fmt"
	"path/filepath"

	"github.com/form3tech/k8s-promoter/internal/clusterconf"
	"github.com/go-git/go-billy/v5"
	"github.com/sirupsen/logrus"
)

const (
	ConfigContent        = "# Please add tenant configuration\n"
	ClusterKustomization = `---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
generatorOptions:
    disableNameSuffixHash: true
`
)

type ConfigKust struct {
	logger   *logrus.Entry
	clusters clusterconf.Clusters
}

func NewConfigKust(log *logrus.Entry, clusters clusterconf.Clusters) *ConfigKust {
	return &ConfigKust{
		logger:   log,
		clusters: clusters,
	}
}

func (k *ConfigKust) Write(fs billy.Filesystem, perCluster map[string][]string) error {
	for _, cluster := range k.clusters {
		workloads, ok := perCluster[cluster.Name()]
		if !ok {
			continue
		}

		if err := k.writeWorkloadConfig(fs, cluster, workloads); err != nil {
			return err
		}

		kustomizationPath := filepath.Join(cluster.ConfigFolder(), KustomizationFile)
		if err := k.writeKustomization(fs, kustomizationPath); err != nil {
			return err
		}
	}

	k.logger.Info("Added kustomization.yaml and config files for new cluster")
	return nil
}

func (k *ConfigKust) writeWorkloadConfig(fs billy.Filesystem, cluster clusterconf.Cluster, workloads []string) error {
	for _, workload := range workloads {
		configPath := filepath.Join(cluster.ConfigFolder(), fmt.Sprintf("%s-config.yaml", workload))

		// only create config-file is it does not exist
		if _, err := fs.Stat(configPath); err == nil {
			return nil
		}
		if err := k.save(fs, configPath, ConfigContent); err != nil {
			return err
		}
	}
	return nil
}

func (k *ConfigKust) writeKustomization(fs billy.Filesystem, kustomizationPath string) error {
	// only create kustomization file if it does not exist
	if _, err := fs.Stat(kustomizationPath); err == nil {
		return nil
	}
	return k.save(fs, kustomizationPath, ClusterKustomization)
}

func (k *ConfigKust) save(fs billy.Filesystem, path string, content string) error {
	f, err := fs.Create(path)
	if err != nil {
		return err
	}
	if _, err := f.Write([]byte(content)); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("f.Close: %w", err)
	}
	return nil
}

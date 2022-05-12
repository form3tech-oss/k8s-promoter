package kustomization_test

import (
	"path/filepath"
	"testing"

	"github.com/form3tech/k8s-promoter/internal/clusterconf"
	"github.com/form3tech/k8s-promoter/internal/kustomization"
	"github.com/form3tech/k8s-promoter/internal/testutils"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestNewConfigKust(t *testing.T) {
	tests := map[string]struct {
		createFiles           bool
		content               string
		expectedConfig        string
		expectedKustomization string
	}{
		"files are created if not present": {
			createFiles:           false,
			expectedConfig:        kustomization.ConfigContent,
			expectedKustomization: kustomization.ClusterKustomization,
		},
		"files retain original content if present": {
			createFiles:           true,
			content:               "don't change me",
			expectedConfig:        "don't change me",
			expectedKustomization: "don't change me",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cluster := clusterconf.Cluster{
				Metadata: clusterconf.ClusterMetadata{
					Name: "dev1",
					Labels: clusterconf.Labels{
						"environment": "development",
						"cloud":       "cloud1",
					},
				},
				Spec: clusterconf.ClusterSpec{
					ConfigFolder: "/config/development/dev1/cloud1",
				},
			}

			fs := memfs.New()
			k := setupKustomization(cluster)

			if tt.createFiles && tt.content != "" {
				testutils.WriteFile(t, fs, path(cluster, "kustomization.yaml"), tt.content)
				testutils.WriteFile(t, fs, path(cluster, "foo-config.yaml"), tt.content)
				testutils.WriteFile(t, fs, path(cluster, "bar-config.yaml"), tt.content)
			}

			clusterWorkloads := map[string][]string{"dev1": []string{"foo", "bar"}}
			err := k.Write(fs, clusterWorkloads)
			require.NoError(t, err)

			testutils.FileHasContents(t, fs, path(cluster, "kustomization.yaml"), tt.expectedKustomization)
			testutils.FileHasContents(t, fs, path(cluster, "foo-config.yaml"), tt.expectedConfig)
			testutils.FileHasContents(t, fs, path(cluster, "bar-config.yaml"), tt.expectedConfig)
		})
	}
}

func path(c clusterconf.Cluster, file string) string {
	return filepath.Join(c.ConfigFolder(), file)
}

func setupKustomization(cluster clusterconf.Cluster) *kustomization.ConfigKust {
	l := logrus.NewEntry(logrus.New())
	return kustomization.NewConfigKust(l, clusterconf.Clusters{cluster})
}

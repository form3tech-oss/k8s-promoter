package clusterconf

import (
	"os"
	"strings"
	"testing"

	"github.com/form3tech/k8s-promoter/internal/environment"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sampleClusters = Clusters{
	Cluster{
		Metadata: ClusterMetadata{
			Name: "dev2-cloud1",
			Labels: Labels{
				"environment": "development",
				"cloud":       "cloud1",
				"stackID":     "dev2-cloud1",
			},
		},
	},
	Cluster{
		Metadata: ClusterMetadata{
			Name: "dev5-cloud2",
			Labels: Labels{
				"environment": "development",
				"cloud":       "cloud2",
				"stackID":     "dev5-cloud2",
			},
		},
	},
	Cluster{
		Metadata: ClusterMetadata{
			Name: "dev4-cloud3",
			Labels: Labels{
				"environment": "development",
				"cloud":       "cloud3",
				"stackID":     "dev4-cloud3",
			},
		},
	},
	Cluster{
		Metadata: ClusterMetadata{
			Name: "test1-cloud1",
			Labels: Labels{
				"environment": "test",
				"cloud":       "cloud1",
				"stackID":     "test1-cloud1",
			},
		},
	},
	Cluster{
		Metadata: ClusterMetadata{
			Name: "prod1-cloud2",
			Labels: Labels{
				"environment": "production",
				"cloud":       "cloud2",
				"stackID":     "prod1-cloud2",
			},
		},
	},
}

func TestClusters_Filter_ByEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		clusters Clusters
		env      environment.Env
		want     Clusters
	}{
		{
			"original_empty",
			Clusters{},
			environment.Development,
			nil,
		},
		{
			"filter_by_development",
			sampleClusters,
			environment.Development,
			Clusters{
				Cluster{
					Metadata: ClusterMetadata{
						Name: "dev2-cloud1",
						Labels: Labels{
							"environment": "development",
							"cloud":       "cloud1",
							"stackID":     "dev2-cloud1",
						},
					},
				},
				Cluster{
					Metadata: ClusterMetadata{
						Name: "dev5-cloud2",
						Labels: Labels{
							"environment": "development",
							"cloud":       "cloud2",
							"stackID":     "dev5-cloud2",
						},
					},
				},
				Cluster{
					Metadata: ClusterMetadata{
						Name: "dev4-cloud3",
						Labels: Labels{
							"environment": "development",
							"cloud":       "cloud3",
							"stackID":     "dev4-cloud3",
						},
					},
				},
			},
		},
		{
			"filter_by_test",
			sampleClusters,
			environment.Test,
			Clusters{
				Cluster{
					Metadata: ClusterMetadata{
						Name: "test1-cloud1",
						Labels: Labels{
							"environment": "test",
							"cloud":       "cloud1",
							"stackID":     "test1-cloud1",
						},
					},
				},
			},
		},
		{
			"filter_by_production",
			sampleClusters,
			environment.Production,
			Clusters{
				Cluster{
					Metadata: ClusterMetadata{
						Name: "prod1-cloud2",
						Labels: Labels{
							"environment": "production",
							"cloud":       "cloud2",
							"stackID":     "prod1-cloud2",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.clusters.Filter(ByEnvironment(tt.env))
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_parseClusters(t *testing.T) {
	tests := []struct {
		fileConfig string
		expConfig  Clusters
	}{
		{
			"testdata/clusters/empty.yaml",
			Clusters{},
		},
		{
			"testdata/clusters/many.yaml",
			Clusters{
				Cluster{
					Version:    "v0.1",
					ConfigType: "Cluster",
					Metadata: ClusterMetadata{
						Name: "dev4",
						Labels: Labels{
							"environment": "development",
							"cloud":       "cloud1",
						},
					},
					Spec: ClusterSpec{
						ManifestFolder: Path("/promoted/development/dev4/cloud1"),
					},
				},
				Cluster{
					Version:    "v0.1",
					ConfigType: "Cluster",
					Metadata: ClusterMetadata{
						Name: "test1",
						Labels: Labels{
							"environment": "test",
							"cloud":       "cloud1",
						},
					},
					Spec: ClusterSpec{
						ManifestFolder: Path("/promoted/test/test1/cloud1"),
					},
				},
				Cluster{
					Version:    "v0.1",
					ConfigType: "Cluster",
					Metadata: ClusterMetadata{
						Name: "prod1",
						Labels: Labels{
							"environment": "production",
							"cloud":       "cloud1",
						},
					},
					Spec: ClusterSpec{
						ManifestFolder: Path("/promoted/production/prod1/cloud1"),
					},
				},
			},
		},
		{
			"testdata/clusters/single.yaml",
			Clusters{
				Cluster{
					Version:    "v0.1",
					ConfigType: "Cluster",
					Metadata: ClusterMetadata{
						Name: "dev4",
						Labels: Labels{
							"environment": "development",
							"cloud":       "cloud1",
						},
					},
					Spec: ClusterSpec{
						ManifestFolder: Path("/promoted/development/dev4/cloud1"),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(testName(tt.fileConfig), func(t *testing.T) {
			f, err := os.ReadFile(tt.fileConfig)
			require.NoError(t, err)

			got, err := ParseClusters(strings.NewReader(string(f)))

			require.NoError(t, err)
			assert.Equal(t, tt.expConfig, got)
		})
	}
}

func testName(file string) string {
	file = strings.ReplaceAll(file, "testdata/", "")
	return strings.ReplaceAll(file, ".yaml", "")
}

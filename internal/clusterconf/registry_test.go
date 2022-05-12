package clusterconf

import (
	"testing"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_FSRegistry_Get(t *testing.T) {
	tests := []struct {
		workload Workload
	}{
		{
			Workload{
				Version:    "v0.1",
				ConfigType: "Workload",
				Metadata: WorkloadMetadata{
					Name:        "workload",
					Description: "A workload",
				},
			},
		},
		{
			Workload{
				Version:    "v0.1",
				ConfigType: "Workload",
				Metadata: WorkloadMetadata{
					Name: "workload-without-config",
				},
			},
		},
		{
			Workload{
				Version:    "v0.1",
				ConfigType: "Workload",
				Metadata: WorkloadMetadata{
					Name:        "workload-with-exclusions",
					Description: "A workload",
				},
				Spec: WorkloadSpec{
					Exclusions: []Exclusion{
						{
							Key:      "cloud",
							Operator: "NotEqual",
							Value:    "cloud1",
						},
						{
							Key:      "environment",
							Operator: "Equal",
							Value:    "development",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(testName(tt.workload.Name()), func(t *testing.T) {
			log := logrus.NewEntry(logrus.New())
			registry := NewWorkloadRegistry(osfs.New("."), "testdata/workloads", log)

			workload, err := registry.Get(tt.workload.Name())
			require.NoError(t, err)
			assert.Equal(t, tt.workload, workload)
		})
	}
}

func Test_FSRegistry_GetAll(t *testing.T) {
	log := logrus.NewEntry(logrus.New())
	registry := NewWorkloadRegistry(osfs.New("."), "testdata/workloads", log)

	got, err := registry.GetAll()
	require.NoError(t, err)
	require.Len(t, got, 3)

	w1, err := registry.Get("workload")
	require.NoError(t, err)
	w2, err := registry.Get("workload-with-exclusions")
	require.NoError(t, err)
	w3, err := registry.Get("workload-without-config")
	require.NoError(t, err)

	want := []Workload{w1, w2, w3}
	assert.ElementsMatch(t, want, got)
}

func Test_FSRegistry_ErrorCases(t *testing.T) {
	tests := []struct {
		name string
		file string
		err  string
	}{
		{
			"empty",
			"testdata/workloads-error-cases/empty/workload.yaml",
			"error loading workload `empty`: could not decode workload config file: EOF",
		},
		{
			"workload-with-invalid-exclusion",
			"testdata/workloads-error-cases/workload-with-invalid-exclusion/workload.yaml",
			"error loading workload `workload-with-invalid-exclusion`: unknown operator: foo",
		},
	}

	for _, tt := range tests {
		t.Run(testName(tt.file), func(t *testing.T) {
			log := logrus.NewEntry(logrus.New())
			registry := NewWorkloadRegistry(osfs.New("."), "testdata/workloads-error-cases", log)

			got, err := registry.Get(tt.name)
			assert.EqualError(t, err, tt.err)
			require.Equal(t, tt.name, got.Name())
			require.Error(t, err)
		})
	}
}

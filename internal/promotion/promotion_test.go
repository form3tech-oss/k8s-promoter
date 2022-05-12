package promotion_test

import (
	"testing"

	"github.com/form3tech/k8s-promoter/internal/detect"
	"github.com/form3tech/k8s-promoter/internal/promotion"
	"github.com/stretchr/testify/require"
)

func Test_PromotionResults(t *testing.T) {
	tests := map[string]struct {
		results             promotion.Results
		clusters            []string
		workloads           []string
		workloadsPerCluster map[string][]string
	}{
		"when not empty": {
			results: promotion.Results{
				"dev1": {
					"foo": detect.WorkloadChange{
						W: detect.Workload{Name: "foo"},
					},
					"bar": detect.WorkloadChange{
						W: detect.Workload{Name: "bar"},
					},
				},
				"dev4": {
					"foo": detect.WorkloadChange{
						W: detect.Workload{Name: "foo"},
					},
					"baz": detect.WorkloadChange{
						W: detect.Workload{Name: "baz"},
					},
				},
			},
			workloads: []string{"bar", "baz", "foo"},
			clusters:  []string{"dev1", "dev4"},
			workloadsPerCluster: map[string][]string{
				"dev1": []string{"bar", "foo"},
				"dev4": []string{"baz", "foo"},
			},
		},
		"when empty": {
			results:             promotion.Results{},
			workloads:           []string{},
			clusters:            []string{},
			workloadsPerCluster: make(map[string][]string),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.ElementsMatch(t, tt.clusters, tt.results.ClusterNames())
			require.ElementsMatch(t, tt.workloads, tt.results.WorkloadNames())
			require.Equal(t, tt.workloadsPerCluster, tt.results.WorkloadsPerCluster())
		})
	}
}

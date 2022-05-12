package promotion

import (
	"sort"

	"github.com/form3tech/k8s-promoter/internal/detect"
)

const (
	NoChangesMsg = "No detected changes match our source environment. Not taking any action"
)

type Kind string

const (
	ManifestUpdate Kind = "manifests_updated"
	NewCluster     Kind = "new_cluster_detected"
)

// Results holds the result of promotions. This structure of this:
// map[cluster]map[workload]detect.WorkloadChange.
type Results map[string]map[string]detect.WorkloadChange

func (promotions Results) ClusterNames() []string {
	var clusters []string
	for cluster := range promotions {
		clusters = append(clusters, cluster)
	}

	sort.Strings(clusters)
	return clusters
}

func (promotions Results) WorkloadNames() []string {
	namesMap := make(map[string]bool)
	for _, workloadChanges := range promotions {
		for _, w := range workloadChanges {
			namesMap[w.W.Name] = true
		}
	}

	var names []string
	for name := range namesMap {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (promotions Results) WorkloadsPerCluster() map[string][]string {
	workloadsPerCluster := make(map[string][]string, len(promotions))
	clusterNames := promotions.ClusterNames()
	for _, cluster := range clusterNames {
		var workloads []string
		for workload := range promotions[cluster] {
			workloads = append(workloads, workload)
		}

		sort.Strings(workloads)
		workloadsPerCluster[cluster] = append(workloadsPerCluster[cluster], workloads...)
	}
	return workloadsPerCluster
}

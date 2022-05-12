package detect

import (
	"sort"
)

type Operation string

const (
	OperationCopy   Operation = "Copy"
	OperationRemove Operation = "Remove"
)

// WorkloadChange represents a change to be conducted over a given workload.
type WorkloadChange struct {
	Op Operation
	W  Workload
}

type (
	WorkloadChanges []WorkloadChange
	FilterFn        func(wc WorkloadChange) bool
)

func (wc WorkloadChanges) Filter(predicate FilterFn) WorkloadChanges {
	var filtered WorkloadChanges
	for _, cluster := range wc {
		if predicate(cluster) {
			filtered = append(filtered, cluster)
		}
	}

	return filtered
}

func (wc WorkloadChanges) NotEmpty() bool {
	return !wc.Empty()
}

func (wc WorkloadChanges) Empty() bool {
	return len(wc) == 0
}

// Workload represents a given workload in a source environment
// e.g. SourceEnv: 'manifests', 'development'...
// SourceEnv is which environment the diff is observed in. i.e the merged PRs changes that triggered a promotion.
type Workload struct {
	SourceEnv string
	Name      string
}

type workloadChangeList []WorkloadChange

func (w workloadChangeList) Len() int {
	return len(w)
}

func (w workloadChangeList) Less(i, j int) bool {
	return w[i].W.Name < w[j].W.Name
}

func (w workloadChangeList) Swap(i, j int) {
	w[i], w[j] = w[j], w[i]
}

func Distinct(changes ...WorkloadChange) WorkloadChanges {
	var distinct WorkloadChanges
	seen := make(map[WorkloadChange]bool)

	for _, change := range changes {
		if _, ok := seen[change]; !ok {
			seen[change] = true
			distinct = append(distinct, change)
		}
	}

	// as traversing maps are not deterministic
	sort.Sort(workloadChangeList(distinct))

	return distinct
}

package detect

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/sirupsen/logrus"
)

const (
	sourceManifestDirLevel = 4
	promotedDirLevel       = 7

	fluxDir           = "flux"
	sourceManifestDir = "manifests"
	promotedDir       = "promoted"
)

var ErrNotWorkloadManifest = fmt.Errorf("not a workload manifest")

type Inferer struct {
	repo         *git.Repository
	commitPrefix string
	logger       *logrus.Entry
}

func NewInferer(repo *git.Repository, commitPrefix string, log *logrus.Entry) *Inferer {
	return &Inferer{
		repo:         repo,
		commitPrefix: commitPrefix,
		logger:       log.WithField("module", "Inferer"),
	}
}

// WorkloadChanges function takes native go-git object.Change, which is unaware of workload/operations
// and converts them into WorkloadChanges slice that carry information about which workload has changed
// and which operation (copy/remove) we need to apply on it.
func (w *Inferer) WorkloadChanges(change *object.Change) ([]WorkloadChange, error) {
	var changes []WorkloadChange

	additions, err := w.additions(change)
	if err != nil {
		return nil, err
	}
	changes = append(changes, additions...)

	deletions, err := w.deletions(change)
	if err != nil {
		return nil, err
	}
	changes = append(changes, deletions...)

	modifications, err := w.modifications(change)
	if err != nil {
		return nil, err
	}
	changes = append(changes, modifications...)

	renames, err := w.renames(change)
	if err != nil {
		return nil, err
	}
	changes = append(changes, renames...)

	return changes, nil
}

func (w *Inferer) additions(change *object.Change) ([]WorkloadChange, error) {
	var additions []WorkloadChange

	if isAddition(change) {
		workload, err := w.workload(change.To.Name)

		if errors.Is(err, ErrNotWorkloadManifest) {
			return additions, nil
		}

		if err != nil {
			return nil, err
		}

		additions = append(additions, WorkloadChange{Op: OperationCopy, W: workload})
	}

	return additions, nil
}

func (w *Inferer) deletions(change *object.Change) ([]WorkloadChange, error) {
	var deletions []WorkloadChange

	if isDeletion(change) {
		workload, err := w.workload(change.From.Name)

		if errors.Is(err, ErrNotWorkloadManifest) {
			return deletions, nil
		}

		if err != nil {
			return nil, err
		}

		// We only deduce a deletion if the workload doesn't exist anymore in either
		// flux/manifests as well as the source environment (if it is a promoted environment)
		var op Operation = OperationCopy
		exists, err := w.workloadExists(workload)
		if err != nil {
			return nil, err
		}
		if !exists {
			op = OperationRemove
		}
		deletions = append(deletions, WorkloadChange{Op: op, W: workload})
	}

	return deletions, nil
}

func (w *Inferer) modifications(change *object.Change) ([]WorkloadChange, error) {
	var modifications []WorkloadChange

	if isModification(change) {
		workload, err := w.workload(change.From.Name)

		if errors.Is(err, ErrNotWorkloadManifest) {
			return modifications, nil
		}

		if err != nil {
			return nil, err
		}
		modifications = append(modifications, WorkloadChange{Op: OperationCopy, W: workload})
	}

	return modifications, nil
}

func (w *Inferer) renames(change *object.Change) ([]WorkloadChange, error) {
	var renames []WorkloadChange

	fromWorkload, err := w.workload(change.From.Name)
	if errors.Is(err, ErrNotWorkloadManifest) {
		return renames, nil
	}
	if err != nil {
		return nil, err
	}

	toWorkload, err := w.workload(change.To.Name)
	if errors.Is(err, ErrNotWorkloadManifest) {
		return renames, nil
	}
	if err != nil {
		return nil, err
	}

	// when rename of manifest is within the same workload
	if isRename(change) && fromWorkload == toWorkload {
		renames = append(renames, WorkloadChange{Op: OperationCopy, W: toWorkload})
	}

	fromWorkloadExists, err := w.workloadExists(fromWorkload)
	if err != nil {
		return nil, err
	}

	// when moving manifests between workloads
	if isRename(change) && fromWorkload != toWorkload && fromWorkloadExists {
		wcs := []WorkloadChange{
			{Op: OperationCopy, W: fromWorkload},
			{Op: OperationCopy, W: toWorkload},
		}
		renames = append(renames, wcs...)
	}

	// when renaming a whole workload
	if isRename(change) && fromWorkload != toWorkload && !fromWorkloadExists {
		wcs := []WorkloadChange{
			{Op: OperationRemove, W: fromWorkload},
			{Op: OperationCopy, W: toWorkload},
		}
		renames = append(renames, wcs...)
	}

	return renames, nil
}

func isAddition(c *object.Change) bool {
	return c.From.Name == "" && c.To.Name != ""
}

func isDeletion(c *object.Change) bool {
	return c.From.Name != "" && c.To.Name == ""
}

func isModification(c *object.Change) bool {
	return c.From.Name != "" && c.To.Name != "" && c.From.Name == c.To.Name
}

func isRename(c *object.Change) bool {
	return c.From.Name != "" && c.To.Name != "" && c.From.Name != c.To.Name
}

func (w *Inferer) workloadExists(workload Workload) (bool, error) {
	exists, err := w.workloadExistsEnv(workload.Name, sourceManifestDir)
	if err != nil {
		return false, err
	}

	if !exists {
		return false, nil
	}

	return true, nil
}

func (w *Inferer) workloadExistsEnv(workloadName, env string) (bool, error) {
	to, err := w.repo.ResolveRevision(plumbing.Revision(w.commitPrefix))
	if err != nil {
		return false, fmt.Errorf("d.Repo.ResolveRevision: %w", err)
	}

	commit, err := w.repo.CommitObject(*to)
	if err != nil {
		return false, fmt.Errorf("d.Repo.CommitObject: %w", err)
	}

	t, err := commit.Tree()
	if err != nil {
		return false, fmt.Errorf("commit.Tree: %w", err)
	}

	path := filepath.Join(fluxDir, env, workloadName)
	_, err = t.Tree(path)
	if errors.Is(err, object.ErrDirectoryNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("t.Tree: %s: %w", path, err)
	}

	return true, nil
}

func (w *Inferer) workload(path string) (Workload, error) {
	if strings.HasPrefix(path, filepath.Join(fluxDir, sourceManifestDir)) {
		return w.inferSourceManifestWorkload(path)
	}

	if strings.HasPrefix(path, filepath.Join(fluxDir, promotedDir)) {
		return w.inferPromotedWorkload(path)
	}

	return Workload{}, fmt.Errorf("%s: %w", path, ErrNotWorkloadManifest)
}

// directory path convention is the following
// flux/manifests/workload/asset.yaml.
func (w *Inferer) inferSourceManifestWorkload(path string) (Workload, error) {
	split := strings.Split(path, "/")
	if len(split) < sourceManifestDirLevel {
		return Workload{}, fmt.Errorf("path: %s: %w", path, ErrUnknownPathConvention)
	}

	return Workload{
		SourceEnv: split[1],
		Name:      split[2],
	}, nil
}

// directory path convention for the promoted workloads is the following
// flux/promoted/environment/cluster/cloud/workload/asset.yaml
// we ignore changes that don't fall within a workload, such as cluster level kustomizations as they are generated
// flux/promoted/development/dev1/cloud1/kustomization.yaml
// if we change the format, we would have to no-op push a promotion until we have a way to signal such a workflow.
func (w *Inferer) inferPromotedWorkload(path string) (Workload, error) {
	split := strings.Split(path, "/")

	if len(split) < promotedDirLevel {
		return Workload{}, fmt.Errorf("path: %s: %w", path, ErrNotWorkloadManifest)
	}

	env := split[2]
	name := split[5]

	return Workload{
		SourceEnv: env,
		Name:      name,
	}, nil
}

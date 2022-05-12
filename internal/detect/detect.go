package detect

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/form3tech/k8s-promoter/internal/clusterconf"
	"github.com/form3tech/k8s-promoter/internal/environment"
	gitint "github.com/form3tech/k8s-promoter/internal/git"
	"github.com/form3tech/k8s-promoter/internal/github"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/sirupsen/logrus"
)

var (
	ErrRepoNotInitialised    = errors.New("repo reference is nil")
	ErrUnknownPathConvention = errors.New("unknown path convention")
	ErrNoChange              = errors.New("no change detected")
)

type Detect struct {
	Repo     *git.Repository
	Inferer  *Inferer
	Registry clusterconf.WorkloadRegistry

	CR     *gitint.CommitRange
	logger *logrus.Entry
}

func NewDetect(repo *git.Repository, commitRange *gitint.CommitRange, registry clusterconf.WorkloadRegistry, log *logrus.Entry) (*Detect, error) {
	if repo == nil {
		return nil, ErrRepoNotInitialised
	}

	inferer := NewInferer(repo, commitRange.ToPrefix, log)
	return &Detect{
		Repo:     repo,
		Inferer:  inferer,
		Registry: registry,
		CR:       commitRange,
		logger:   log.WithField("module", "Detect"),
	}, nil
}

// WorkloadChange generates a slice of WorkloadChange from go-git changes
// This is how we understand what workload has changed and what the operation is required, i.e. copy or delete.
func (d *Detect) WorkloadChange() ([]WorkloadChange, error) {
	from, err := d.Repo.ResolveRevision(plumbing.Revision(d.CR.FromPrefix))
	if err != nil {
		return nil, fmt.Errorf("repo.ResolveRevision: %w", err)
	}

	to, err := d.Repo.ResolveRevision(plumbing.Revision(d.CR.ToPrefix))
	if err != nil {
		return nil, fmt.Errorf("repo.ResolveRevision: %w", err)
	}

	diffs, err := diffTree(d.Repo, from, to)
	if err != nil {
		return nil, err
	}

	changes, err := d.processDiffs(diffs)
	if err != nil {
		return nil, err
	}

	if len(changes) == 0 {
		return nil, fmt.Errorf("from: %s, to: %s: %w", d.CR.FromPrefix, d.CR.ToPrefix, ErrNoChange)
	}

	return changes, nil
}

// NewClusterWorkloads generates a slice or WorkloadChange to be promoted to new clusters.
// For development, we look at flux/manifests to figure out what to promote.
// For test/production, we look into previous environment to figure out all workloads.
func (d *Detect) NewClusterWorkloads(targetEnv environment.Env, previousEnvClusters clusterconf.Clusters) ([]WorkloadChange, error) {
	source, err := targetEnv.ManifestSource()
	if err != nil {
		return nil, err
	}

	if targetEnv == environment.Development {
		return d.fromManifestSource(source)
	}

	return d.fromClustersInPreviousEnv(source, previousEnvClusters)
}

func (d *Detect) fromManifestSource(sourceEnv environment.Env) ([]WorkloadChange, error) {
	var changes []WorkloadChange

	workloads, err := d.Registry.GetAll()
	if err != nil {
		return changes, err
	}

	for _, workload := range workloads {
		change := WorkloadChange{
			Op: OperationCopy,
			W: Workload{
				Name:      workload.Name(),
				SourceEnv: string(sourceEnv),
			},
		}
		changes = append(changes, change)
	}

	return changes, err

}

func (d *Detect) fromClustersInPreviousEnv(sourceEnv environment.Env, previousEnvClusters clusterconf.Clusters) ([]WorkloadChange, error) {
	wt, err := d.Repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("NewClusterWorkloads: %w", err)
	}

	promotedWorkloads := make(map[string]struct{})
	for _, cluster := range previousEnvClusters {
		fileInfos, err := wt.Filesystem.ReadDir(cluster.ManifestFolder())
		if err != nil {
			return nil, err
		}

		for _, fileInfo := range fileInfos {
			if !fileInfo.IsDir() {
				continue
			}

			promotedWorkloads[fileInfo.Name()] = struct{}{}
		}
	}

	changes := make([]WorkloadChange, 0, len(promotedWorkloads))
	for workload := range promotedWorkloads {
		change := WorkloadChange{
			Op: OperationCopy,
			W: Workload{
				Name:      workload,
				SourceEnv: string(sourceEnv),
			},
		}
		changes = append(changes, change)
	}

	return changes, nil
}

func (d *Detect) processDiffs(diffs object.Changes) (WorkloadChanges, error) {
	var wc []WorkloadChange

	for _, gitChange := range diffs {
		changes, err := d.Inferer.WorkloadChanges(gitChange)
		if err != nil {
			return nil, err
		}

		wc = append(wc, changes...)
	}

	return Distinct(wc...), nil
}

func (d *Detect) GetSourceCommits() ([]*github.Commit, error) {
	sourceCommits := make([]*github.Commit, 0)

	from, err := d.Repo.ResolveRevision(plumbing.Revision(d.CR.FromPrefix))
	if err != nil {
		return sourceCommits, fmt.Errorf("repo.ResolveRevision: %w", err)
	}

	to, err := d.Repo.ResolveRevision(plumbing.Revision(d.CR.ToPrefix))
	if err != nil {
		return sourceCommits, fmt.Errorf("repo.ResolveRevision: %w", err)
	}
	d.logger.Infof("Searching for source commit tags in range %s...%s", from.String(), to.String())
	regex := regexp.MustCompile(`Source-commit: (.*) A:(.*) C:([^\r\n]*)`)

	// Get merge commit and iterator of pre merge. Filter out pre merge commits.
	preCommit, err := d.Repo.CommitObject(*from)
	if err != nil {
		return sourceCommits, fmt.Errorf("repo.Log: %w", err)
	}
	postCommit, err := d.Repo.CommitObject(*to)
	if err != nil {
		return sourceCommits, fmt.Errorf("repo.CommitObject: %w", err)
	}

	excludeIter, err := d.Repo.Log(&git.LogOptions{From: *from})
	if err != nil {
		return sourceCommits, fmt.Errorf("repo.Log: %w", err)
	}

	seen := map[plumbing.Hash]struct{}{}
	err = excludeIter.ForEach(func(c *object.Commit) error {
		seen[c.Hash] = struct{}{}
		return nil
	})
	if err != nil {
		return sourceCommits, fmt.Errorf("excludeIter.ForEach: %w", err)
	}

	var isValid object.CommitFilter = func(c *object.Commit) bool {
		_, ok := seen[c.Hash]
		return !ok && len(c.ParentHashes) < 2
	}
	var stop object.CommitFilter = func(c *object.Commit) bool {
		return c.Hash == preCommit.Hash
	}
	iter := object.NewFilterCommitIter(postCommit, &isValid, &stop)
	err = iter.ForEach(func(c *object.Commit) error {
		matches := regex.FindAllStringSubmatch(c.Message, -1)
		d.logger.Infof("Found commit %s with %d source commits", c.Hash.String(), len(matches))
		for _, match := range matches {
			sourceCommits = append(sourceCommits, &github.Commit{
				Hash:           match[1],
				AuthorLogin:    match[2],
				CommitterLogin: match[3],
			})
		}
		return nil
	})
	if err != nil {
		return sourceCommits, fmt.Errorf("newFilterCommitIter.ForEach: %w", err)
	}

	return sourceCommits, nil
}

func diffTree(repo *git.Repository, from, to *plumbing.Hash) (object.Changes, error) {
	fromTree, err := tree(repo, from)
	if err != nil {
		return nil, err
	}

	toTree, err := tree(repo, to)
	if err != nil {
		return nil, err
	}

	c, err := fromTree.Diff(toTree)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func tree(repo *git.Repository, hash *plumbing.Hash) (*object.Tree, error) {
	commit, err := repo.CommitObject(*hash)
	if err != nil {
		return nil, fmt.Errorf("repo.CommitObject: %w", err)
	}

	t, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("commit.Tree: %w", err)
	}

	return t, nil
}

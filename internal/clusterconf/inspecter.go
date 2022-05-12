package clusterconf

import (
	"errors"
	"fmt"
	"os"

	"github.com/form3tech/k8s-promoter/internal/environment"
	"github.com/go-git/go-git/v5"
	"github.com/sirupsen/logrus"
)

var ErrRepoNotInitialised = errors.New("repo reference is nil")

type ClusterInspecter struct {
	Repo   *git.Repository
	logger *logrus.Entry
}

type ClusterDetection struct {
	All         Clusters
	New         Clusters
	Existing    Clusters
	PreviousEnv Clusters
}

func NewClusterInspecter(repo *git.Repository, log *logrus.Entry) (*ClusterInspecter, error) {
	if repo == nil {
		return nil, ErrRepoNotInitialised
	}

	return &ClusterInspecter{Repo: repo, logger: log.WithField("module", "ClusterInspector")}, nil
}

// Detect analyses cluster config and local repository directory structure to work out newly added clusters,
// existing clusters and clusters belonging to the previous environment.
func (c *ClusterInspecter) Detect(all Clusters, targetEnv environment.Env) (ClusterDetection, error) {
	inEnvironment := all.Filter(ByEnvironment(targetEnv))
	new, err := c.newClusters(inEnvironment)
	if err != nil {
		return ClusterDetection{}, err
	}

	existing := c.existingClusters(inEnvironment, new)
	previous := c.fromPreviousEnv(all, targetEnv)

	return ClusterDetection{
		All:         all,
		New:         new,
		Existing:    existing,
		PreviousEnv: previous,
	}, nil
}

func (c *ClusterInspecter) newClusters(all Clusters) ([]Cluster, error) {
	var missingOnDisk []Cluster

	wt, err := c.Repo.Worktree()
	if err != nil {
		return missingOnDisk, fmt.Errorf("worktree: %w", err)
	}

	for _, cluster := range all {
		_, err := wt.Filesystem.Stat(cluster.ManifestFolder())
		if err != nil {
			if err == os.ErrNotExist {
				missingOnDisk = append(missingOnDisk, cluster)
			} else {
				return nil, err
			}
		}
	}

	return missingOnDisk, nil
}

func (c *ClusterInspecter) existingClusters(all Clusters, new Clusters) []Cluster {
	return all.Filter(Without(new))
}

func (c *ClusterInspecter) fromPreviousEnv(all Clusters, targetEnv environment.Env) []Cluster {
	source, err := targetEnv.ManifestSource()
	if err != nil {
		return []Cluster{}
	}
	return all.Filter(ByEnvironment(source))
}

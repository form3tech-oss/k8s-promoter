package promotion

import (
	"context"
	"fmt"

	"github.com/form3tech/k8s-promoter/internal/clusterconf"
	"github.com/form3tech/k8s-promoter/internal/detect"
	"github.com/form3tech/k8s-promoter/internal/environment"
	"github.com/form3tech/k8s-promoter/internal/github"
	"github.com/form3tech/k8s-promoter/internal/kustomization"
	"github.com/go-git/go-billy/v5"
	"github.com/sirupsen/logrus"
)

// PromotionToNewCluster implements Promotion interface. It encapsulates the logic of finding
// workload changes and clusters when we detect new clusters added in the config repo.
// It contains one additional step that must be done following promotion of manifests to new cluster,
// namely creating config files and kustomization for new cluster. This is currently done in AfterChanges.
type PromotionToNewCluster struct {
	env           environment.Env
	kind          Kind
	sourceCommits []*github.Commit
	assignees     []string

	clusters clusterconf.ClusterDetection
	detect   *detect.Detect
	repo     *github.ManifestRepository

	logger *logrus.Entry
}

func NewPromotionToNewCluster(ctx context.Context, l *logrus.Entry, env environment.Env, r *github.ManifestRepository, d *detect.Detect, c clusterconf.ClusterDetection) (*PromotionToNewCluster, error) {
	return &PromotionToNewCluster{
		env:           env,
		clusters:      c,
		kind:          NewCluster,
		assignees:     []string{},
		sourceCommits: []*github.Commit{},
		detect:        d,
		logger:        l,
		repo:          r,
	}, nil
}

func (s *PromotionToNewCluster) Kind() Kind {
	return s.kind
}

func (s *PromotionToNewCluster) Assignes() []string {
	return s.assignees
}

func (s *PromotionToNewCluster) SourceCommits() []*github.Commit {
	return s.sourceCommits
}

func (s *PromotionToNewCluster) Changes() ([]detect.WorkloadChange, clusterconf.Clusters, error) {
	if len(s.clusters.New) == 0 {
		return []detect.WorkloadChange{}, clusterconf.Clusters{}, nil
	}

	changes, err := s.detect.NewClusterWorkloads(s.env, s.clusters.PreviousEnv)
	if err != nil {
		return []detect.WorkloadChange{}, clusterconf.Clusters{}, fmt.Errorf("promoteAllWorkloadsToNewClusters: %w", err)
	}

	return changes, s.clusters.New, nil
}

func (s *PromotionToNewCluster) AfterChanges(promotions Results, clusters clusterconf.Clusters) error {
	fs, err := s.repo.WorkingTreeFS()
	if err != nil {
		return err
	}

	return s.addNewClusterConfig(fs, promotions, clusters)
}

func (s *PromotionToNewCluster) addNewClusterConfig(fs billy.Filesystem, promotions Results, clusters clusterconf.Clusters) error {
	kust := kustomization.NewConfigKust(s.logger, clusters)

	if err := kust.Write(fs, promotions.WorkloadsPerCluster()); err != nil {
		s.logger.Info("error writing confiuration for new cluster: %w", err)
	}

	return nil
}

package promotion

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/form3tech/k8s-promoter/internal/clusterconf"
	"github.com/form3tech/k8s-promoter/internal/detect"
	"github.com/form3tech/k8s-promoter/internal/environment"
	"github.com/form3tech/k8s-promoter/internal/github"
	"github.com/sirupsen/logrus"
)

// PromotionManifestUpdate implements Promotion interface. It encapsulates the logic of finding
// workload changes and clusters when we detect edited manifests in the commit range.
type PromotionManifestUpdate struct {
	env           environment.Env
	kind          Kind
	sourceCommits []*github.Commit
	assignees     []string

	clusters clusterconf.ClusterDetection
	detect   *detect.Detect
	repo     *github.ManifestRepository

	logger *logrus.Entry
}

func NewPromotionManifestUpdate(ctx context.Context, l *logrus.Entry, env environment.Env, r *github.ManifestRepository, d *detect.Detect, c clusterconf.ClusterDetection) (*PromotionManifestUpdate, error) {
	var sourceCommits []*github.Commit
	var err error

	if env == environment.Development {
		l.Debug("promoting to development: finding authors from GitHub")
		sourceCommits, err = r.GetCommits(ctx, d.CR.FromPrefix, d.CR.ToPrefix)
	} else {
		l.Debugf("promoting to %s: finding source manifest authors and commits from commit messages", env)
		sourceCommits, err = d.GetSourceCommits()
	}

	if err != nil {
		return nil, err
	}

	assignees, err := r.GetPullRequestAssignees(ctx, sourceCommits)
	if err != nil {
		return nil, err
	}

	return &PromotionManifestUpdate{
		env:           env,
		kind:          ManifestUpdate,
		assignees:     assignees,
		sourceCommits: sourceCommits,
		detect:        d,
		logger:        l,
		clusters:      c,
		repo:          r,
	}, nil
}

func (s *PromotionManifestUpdate) Changes() ([]detect.WorkloadChange, clusterconf.Clusters, error) {
	workloadChanges, err := s.detect.WorkloadChange()
	if err != nil && !errors.Is(err, detect.ErrNoChange) {
		return nil, nil, fmt.Errorf("diff: %w", err)
	}

	selectedChanges, err := s.changesFromPreviousEnv(workloadChanges)
	if err != nil {
		return nil, nil, fmt.Errorf("promoteAllWorkloadsToNewClusters: %w", err)
	}

	if len(selectedChanges) == 0 {
		s.logger.WithFields(
			logrus.Fields{
				"promotion_type": s.kind,
				"target_env":     s.env,
			}).Info(NoChangesMsg)
		return []detect.WorkloadChange{}, clusterconf.Clusters{}, nil
	}

	return selectedChanges, s.clusters.Existing, nil

}

func (s *PromotionManifestUpdate) changesFromPreviousEnv(changes []detect.WorkloadChange) ([]detect.WorkloadChange, error) {
	var selected []detect.WorkloadChange

	manifestSource, err := s.env.ManifestSource()
	if err != nil {
		return selected, err
	}

	for _, change := range changes {
		if environment.Env(change.W.SourceEnv) == manifestSource {
			selected = append(selected, change)
		} else {
			s.logger.Infof(
				"dropping change '%s %s' as the source '%s' is not '%s'",
				strings.ToLower(string(change.Op)),
				change.W.Name,
				change.W.SourceEnv,
				manifestSource,
			)
		}
	}
	return selected, nil
}

func (s *PromotionManifestUpdate) AfterChanges(_ Results, _ clusterconf.Clusters) error {
	s.logger.Info("nothing to do")
	return nil
}

func (s *PromotionManifestUpdate) Kind() Kind {
	return s.kind
}

func (s *PromotionManifestUpdate) Assignes() []string {
	return s.assignees
}

func (s *PromotionManifestUpdate) SourceCommits() []*github.Commit {
	return s.sourceCommits
}

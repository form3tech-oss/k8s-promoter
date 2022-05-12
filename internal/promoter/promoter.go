package promoter

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/form3tech/k8s-promoter/internal/clusterconf"
	"github.com/form3tech/k8s-promoter/internal/detect"
	"github.com/form3tech/k8s-promoter/internal/environment"
	"github.com/form3tech/k8s-promoter/internal/filesystem"
	"github.com/form3tech/k8s-promoter/internal/git"
	"github.com/form3tech/k8s-promoter/internal/github"
	"github.com/form3tech/k8s-promoter/internal/kustomization"
	promotion "github.com/form3tech/k8s-promoter/internal/promotion"
	"github.com/go-git/go-billy/v5/util"
	gh "github.com/google/go-github/v33/github"
	"github.com/sirupsen/logrus"
)

const (
	Timeout       = 5 * time.Minute
	NotInSyncMsg  = "Clusters for target environment are out of sync. Not raising further PRs until this is resolved"
	NoClustersMsg = "Found no clusters to promote workload to, please check clusters.yaml if you think this is an error"
	NoChangesMsg  = "No detected changes match our source environment. Not taking any action"
)

var (
	ErrClustersNotInSync  = errors.New("clusters not in sync")
	ErrInvalidEnvironment = errors.New("invalid environment name")
)

type Args struct {
	CloneArgs       *git.CloneArgs
	CommitRange     *git.CommitRange
	AdminRepository string
	TargetEnv       string

	ConfigPath       string
	GPGKeyPath       string
	ConfigRepository string

	CommitterName  string
	CommitterEmail string

	NoIssueUsers []string
}

type Promotion interface {
	Changes() ([]detect.WorkloadChange, clusterconf.Clusters, error)
	AfterChanges(promotion.Results, clusterconf.Clusters) error

	Kind() promotion.Kind
	Assignes() []string
	SourceCommits() []*github.Commit
}

type Promoter struct {
	manifestRepo  *github.ManifestRepository
	detect        *detect.Detect
	kustomization *kustomization.Kust
	prBuilder     *PullRequestBuilder

	registry clusterconf.WorkloadRegistry // providing workload exclusion filtering
	clusters clusterconf.ClusterDetection

	logger *logrus.Entry
}

func NewPromoter(ctx context.Context, args *Args, log *logrus.Entry, ghClient *gh.Client, ghSleep time.Duration) (*Promoter, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()

	repo, err := git.Clone(ctxTimeout, args.CloneArgs)
	if err != nil {
		return nil, fmt.Errorf("git.Clone: %w", err)
	}

	signKey, err := github.ReadSignKey(args.GPGKeyPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read commit signing key: %w", err)
	}
	manifestRepo, err := github.NewManifestRepository(
		github.WithRepository(repo),
		github.WithGitAuth(args.CloneArgs.Auth),
		github.WithGithubRepositoryConfig(github.RepositoryConfig{
			Owner:        args.CloneArgs.Owner,
			Repository:   args.CloneArgs.Repo,
			TargetBranch: args.CloneArgs.Branch,
			TargetRef:    args.CloneArgs.Ref,
		}),
		github.WithGithubClient(ghClient),
		github.WithSignKey(signKey),
		github.WithLogger(log),
		github.WithSleep(ghSleep),
		github.WithCommitter(args.CommitterName, args.CommitterEmail),
		github.WithNoIssueUsers(args.NoIssueUsers),
	)
	if err != nil {
		return nil, err
	}

	fs, err := manifestRepo.WorkingTreeFS()
	if err != nil {
		return nil, err
	}

	builder, err := NewPullRequestBuilder(fs, log, environment.Env(args.TargetEnv))
	if err != nil {
		return nil, fmt.Errorf("descriptionBuilder: %w", err)
	}

	// Explicitly remove the .git file created after clone and checkout.
	// See the test Test_GitAssumptions_StorageUsedForGitCloneCanImpactOnClonedRepoWorktree
	// for an explanation of go-git's behaviour.
	if err := fs.Remove(".git"); err != nil {
		return nil, fmt.Errorf("removing billy.Filesystem's .git: %w", err)
	}

	clustersFromConfigRepo, err := fetchClustersConfig(ctx, ghClient, args)
	if err != nil {
		return nil, err
	}

	workloadRegistry := clusterconf.NewWorkloadRegistry(fs, "flux/manifests", log)

	d, err := detect.NewDetect(repo, args.CommitRange, workloadRegistry, log)
	if err != nil {
		return nil, fmt.Errorf("detect.New: %w", err)
	}

	clusterDetector, err := clusterconf.NewClusterInspecter(repo, log)
	if err != nil {
		return nil, fmt.Errorf("detect.NewCluster: %w", err)
	}

	clusters, err := clusterDetector.Detect(clustersFromConfigRepo, environment.Env(args.TargetEnv))
	if err != nil {
		return nil, fmt.Errorf("clusterconf.ClusterDetection: %w", err)
	}

	promoter := &Promoter{
		manifestRepo:  manifestRepo,
		detect:        d,
		kustomization: kustomization.NewKust(log),
		prBuilder:     builder,
		registry:      workloadRegistry,
		clusters:      clusters,
		logger:        log,
	}
	return promoter, nil
}

func (p *Promoter) Promote(ctx context.Context, env string) error {
	targetEnv := environment.Env(env)
	if err := targetEnv.Validate(); err != nil {
		return ErrInvalidEnvironment
	}

	ctxExisting, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()

	promotionManifests, err := promotion.NewPromotionManifestUpdate(ctxExisting, p.logger, targetEnv, p.manifestRepo, p.detect, p.clusters)
	if err != nil {
		return err
	}
	err = p.promote(ctxExisting, promotionManifests, targetEnv)
	if err != nil {
		if errors.Is(err, ErrClustersNotInSync) {
			p.logger.Info(NotInSyncMsg)
		}
		return err
	}

	ctxNew, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()

	promotionNewCluster, err := promotion.NewPromotionToNewCluster(ctxNew, p.logger, targetEnv, p.manifestRepo, p.detect, p.clusters)
	if err != nil {
		return err
	}
	err = p.promote(ctxNew, promotionNewCluster, targetEnv)
	if err != nil {
		if errors.Is(err, ErrClustersNotInSync) {
			p.logger.Info(NotInSyncMsg)
		}
		return err
	}

	return nil
}

func (p *Promoter) promote(ctx context.Context, promotion Promotion, targetEnv environment.Env) error {
	p.logger.WithFields(
		logrus.Fields{
			"promotion_type": promotion.Kind(),
			"target_env":     targetEnv,
		}).Info("Beginning promotion")

	changes, clusters, err := promotion.Changes()
	if err != nil {
		return err
	}

	if len(clusters) == 0 {
		p.logger.Info(NoClustersMsg)
		return nil
	}

	for _, clustersGroup := range clusters.Group(targetEnv) {
		branchName, err := p.manifestRepo.NewPromoteBranch()
		if err != nil {
			return err
		}

		results, err := p.performChanges(ctx, changes, clustersGroup, targetEnv)
		if err != nil {
			return err
		}

		if len(results) == 0 {
			continue
		}

		if err := promotion.AfterChanges(results, clustersGroup); err != nil {
			return err
		}

		pr := p.prBuilder.Build(results, promotion.SourceCommits(), promotion.Kind())
		err = p.manifestRepo.Commit(pr.CommitMessage)
		if err != nil {
			return err
		}

		err = p.manifestRepo.RaisePromotion(ctx, branchName, pr, promotion.Assignes())
		if err != nil {
			return err
		}
	}
	return nil
}

// performChanges performs change.OP for all changes in each cluster where workload belonging to the change is allowed
// This will change the working tree of the repository i.e. un-staged changes.
func (p *Promoter) performChanges(ctx context.Context, changes []detect.WorkloadChange, clusters clusterconf.Clusters, targetEnv environment.Env) (promotion.Results, error) {
	// A cluster can have workload exclusion and at this point we will omit it (happens in allowedChanges)
	// We need to keep track of which clusters actually received promotion to raise PR only for them
	promotions := make(promotion.Results, len(clusters))

	for _, cluster := range clusters {
		clusterChanges, err := p.allowedChanges(ctx, changes, cluster)
		if err != nil {
			return nil, err
		}

		for _, clusterWorkloadChange := range clusterChanges {
			workload, err := p.registry.Get(clusterWorkloadChange.W.Name)
			if err != nil {
				return nil, fmt.Errorf("registry.Get: %w", err)
			}

			err = p.verifyWorkloadConsistency(workload, environment.Env(clusterWorkloadChange.W.SourceEnv))
			if err != nil {
				return nil, fmt.Errorf("verifyWorkloadConsistency: %w", err)
			}

			err = p.performChange(ctx, cluster, clusterWorkloadChange, targetEnv)
			if err != nil {
				return nil, fmt.Errorf("performChange: %w", err)
			}

			// this tells us what was promoted to where and why (what's the kind of the promotion)
			_, exists := promotions[cluster.Name()]
			if !exists {
				promotions[cluster.Name()] = make(map[string]detect.WorkloadChange)
			}

			promotions[cluster.Name()][workload.Name()] = clusterWorkloadChange
		}
	}

	return promotions, nil
}

// performChange uses previous environment as source for copying workload manifests from.
// As we are checking the consistency of workloads (i.e. all clusters in previous environment are running the same promoted version)
func (p *Promoter) performChange(ctx context.Context, cluster clusterconf.Cluster, change detect.WorkloadChange, targetEnv environment.Env) error {
	if ctx.Err() != nil {
		return fmt.Errorf("performChange: %w", ctx.Err())
	}

	p.logger.WithFields(logrus.Fields{
		"cluster":     cluster.Name(),
		"environment": targetEnv,
		"workload":    change.W.Name,
	}).Info("Promoting workload to cluster")

	fs, err := p.manifestRepo.WorkingTreeFS()
	if err != nil {
		return err
	}

	targetDir := cluster.WorkloadPath(change.W.Name)
	sourceDir, err := p.getSourceDir(change, targetEnv)
	if err != nil {
		return err
	}

	p.logger.WithFields(logrus.Fields{
		"operation": change.Op,
		"sourceDir": sourceDir,
		"targetDir": targetDir,
	}).Debug("applyChange")

	if change.Op != detect.OperationCopy && change.Op != detect.OperationRemove {
		return fmt.Errorf("op not known: %s", change.Op)
	}

	if change.Op == detect.OperationCopy {
		err := filesystem.Replace(fs, sourceDir, targetDir)
		if err != nil {
			return fmt.Errorf("replace dir: %w", err)
		}
	}

	if change.Op == detect.OperationRemove {
		_, err := fs.Stat(targetDir)
		if err != nil {
			return fmt.Errorf("stat: %s: %w", targetDir, err)
		}

		err = util.RemoveAll(fs, targetDir)
		if err != nil {
			return err
		}
	}

	return p.kustomization.Write(fs, cluster)
}

func (p *Promoter) getSourceDir(change detect.WorkloadChange, targetEnv environment.Env) (string, error) {
	manifestsSource, err := targetEnv.ManifestSource()
	if err != nil {
		return "", fmt.Errorf("targetEnv.ManifestSource: %w", err)
	}

	if manifestsSource == environment.SourceManifest {
		// TODO we should provide a better way of constructing these paths
		return clusterconf.Path(filepath.Join(string(environment.SourceManifest), change.W.Name)), nil
	}

	workload, err := p.registry.Get(change.W.Name)
	if err != nil {
		return "", fmt.Errorf("registry.Get: %w", err)
	}

	previousClusters := p.clusters.All.
		Filter(clusterconf.ByAllowWorkload(workload)).
		Filter(clusterconf.ByEnvironment(manifestsSource))

	if len(previousClusters) == 0 {
		p.logger.WithFields(logrus.Fields{
			"workload":     workload,
			"previous_env": manifestsSource,
		}).Error("filtered clusters are zero, expected at least one")

		return "", fmt.Errorf("workload: %s, env: %s: filtered clusters are zero, expected at least one",
			workload, manifestsSource)
	}

	return previousClusters[0].WorkloadPath(change.W.Name), nil
}

// verifyWorkloadConsistency ensures that a workload inside an environment is consistent, meaning that the
// contents of the workload directory are identical. The business requirement is that all PRs
// for an environment are merged before continuing to the next environment.
func (p *Promoter) verifyWorkloadConsistency(workload clusterconf.Workload, manifestSource environment.Env) error {
	if manifestSource == environment.SourceManifest {
		return nil
	}

	previousClusters := p.clusters.All.
		Filter(clusterconf.ByAllowWorkload(workload)).
		Filter(clusterconf.ByEnvironment(manifestSource))

	fs, err := p.manifestRepo.WorkingTreeFS()
	if err != nil {
		return fmt.Errorf("p.manifestRepo.CopyRepoFS: %w", err)
	}

	var clusterNames []string
	hashes := map[string][]string{}

	for _, c := range previousClusters {
		clusterNames = append(clusterNames, c.Name())

		workloadDir := c.WorkloadPath(workload.Name())
		h, err := filesystem.DirHash(fs, workloadDir)
		if err != nil {
			return fmt.Errorf("hash directory %s: %w", workloadDir, err)
		}

		hashes[h] = append(hashes[h], workloadDir)
	}

	if len(hashes) > 1 {
		return fmt.Errorf("workload '%s' differs across clusters %v: %w", workload.Name(), clusterNames, ErrClustersNotInSync)
	}

	return nil
}

func (p *Promoter) allowedChanges(ctx context.Context, changes []detect.WorkloadChange, cluster clusterconf.Cluster) ([]detect.WorkloadChange, error) {
	if ctx.Err() != nil {
		return nil, fmt.Errorf("allowedChanges: %w", ctx.Err())
	}

	var perClusterChanges []detect.WorkloadChange

	for _, change := range changes {
		workload, err := p.registry.Get(change.W.Name)
		if err != nil {
			return nil, fmt.Errorf("p.registry.Get: %w", err)
		}

		if cluster.AllowWorkload(workload) {
			perClusterChanges = append(perClusterChanges, change)
		} else {
			p.logger.WithFields(
				logrus.Fields{
					"cluster":   cluster.Name(),
					"workload":  workload,
					"operation": change.Op,
				}).Infof("workload excluded")
		}
	}

	return perClusterChanges, nil
}

func fetchClustersConfig(ctx context.Context, ghClient *gh.Client, args *Args) (clusterconf.Clusters, error) {
	config, _, _, err := ghClient.Repositories.GetContents(
		ctx,
		args.CloneArgs.Owner,
		args.ConfigRepository,
		args.ConfigPath,
		&gh.RepositoryContentGetOptions{
			Ref: "master",
		},
	)
	if err != nil {
		return clusterconf.Clusters{}, fmt.Errorf("fetching config: %w", err)
	}

	content, err := config.GetContent()
	if err != nil {
		return clusterconf.Clusters{}, fmt.Errorf("decoding config: %w", err)
	}

	clusters, err := clusterconf.ParseClusters(strings.NewReader(content))
	if err != nil {
		return clusterconf.Clusters{}, fmt.Errorf("parse clusters: %w", err)
	}

	return clusters, nil
}

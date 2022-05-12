package github

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v33/github"
	"github.com/sirupsen/logrus"
)

const (
	webFlowUser  = "web-flow"
	prLabel      = "k8s-promoter/automated-promotion"
	maxAssignees = 10
)

var (
	ErrGitHubClientRequired = errors.New("github client required")
	ErrCommitterRequired    = errors.New("committer required")
)

type ManifestRepository struct {
	auth *githttp.BasicAuth

	githubRepositoryConfig RepositoryConfig

	committer *User

	noIssueUsers []string

	client        *github.Client
	repo          *git.Repository
	signKey       *openpgp.Entity
	logger        *logrus.Entry
	sleepDuration time.Duration
}

type RepositoryOption func(r *ManifestRepository)

type User struct {
	Name  string
	Email string
}

type Commit struct {
	Hash           string
	AuthorLogin    string
	CommitterLogin string
}

type PromotionPullRequest struct {
	Title         string
	Description   string
	CommitMessage string
}

func WithGitAuth(auth *githttp.BasicAuth) RepositoryOption {
	return func(r *ManifestRepository) {
		r.auth = auth
	}
}

func WithGithubClient(client *github.Client) RepositoryOption {
	return func(r *ManifestRepository) {
		r.client = client
	}
}

type RepositoryConfig struct {
	Owner        string
	Repository   string
	TargetBranch string
	TargetRef    string
}

func (c RepositoryConfig) RepoURL() string {
	return fmt.Sprintf("https://github.com/%s/%s.git", c.Owner, c.Repository)
}

func WithGithubRepositoryConfig(cfg RepositoryConfig) RepositoryOption {
	return func(r *ManifestRepository) {
		r.githubRepositoryConfig = cfg
	}
}

func WithSignKey(signKey *openpgp.Entity) RepositoryOption {
	return func(r *ManifestRepository) {
		r.signKey = signKey
	}
}

func WithLogger(logger *logrus.Entry) RepositoryOption {
	return func(r *ManifestRepository) {
		r.logger = logger
	}
}

func WithRepository(repo *git.Repository) RepositoryOption {
	return func(r *ManifestRepository) {
		r.repo = repo
	}
}

func WithSleep(sleepDuration time.Duration) RepositoryOption {
	return func(r *ManifestRepository) {
		r.sleepDuration = sleepDuration
	}
}

func WithCommitter(name string, email string) RepositoryOption {
	return func(r *ManifestRepository) {
		r.committer = &User{Name: name, Email: email}
	}
}

func WithNoIssueUsers(users []string) RepositoryOption {
	return func(r *ManifestRepository) {
		r.noIssueUsers = users
	}
}

func NewManifestRepository(opts ...RepositoryOption) (*ManifestRepository, error) {
	r := &ManifestRepository{}
	for _, opt := range opts {
		opt(r)
	}

	if r.client == nil {
		return nil, ErrGitHubClientRequired
	}

	if r.committer == nil {
		return nil, ErrCommitterRequired
	}

	if r.logger == nil {
		r.logger = logrus.NewEntry(logrus.New())
	}

	return r, nil
}

func (r *ManifestRepository) NewPromoteBranch() (string, error) {
	wt, err := r.repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("get repo worktree: %w", err)
	}

	startFrom, err := r.repo.ResolveRevision(plumbing.Revision(r.githubRepositoryConfig.TargetRef))
	if err != nil {
		return "", fmt.Errorf("RaisePromotion: resolve revision: %w", err)
	}

	branchName := fmt.Sprintf("k8s-promoter-%d", time.Now().UnixNano())
	err = wt.Checkout(&git.CheckoutOptions{
		Hash:   *startFrom,
		Branch: plumbing.NewBranchReferenceName(branchName),
		Create: true,
		Force:  true,
	})
	if err != nil {
		return "", fmt.Errorf("checkout new branch: %w", err)
	}

	return branchName, nil
}

func (r *ManifestRepository) WorkingTreeFS() (billy.Filesystem, error) {
	wt, err := r.repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("get repo worktree: %w", err)
	}

	return wt.Filesystem, nil
}

func (r *ManifestRepository) Commit(msg string) error {
	wt, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("get repo worktree: %w", err)
	}

	err = wt.AddGlob(".")
	if err != nil {
		return fmt.Errorf("add . to worktree: %w", err)
	}

	_, err = wt.Commit(msg, &git.CommitOptions{
		All: true,
		Author: &object.Signature{
			Name:  r.committer.Name,
			Email: r.committer.Email,
			When:  time.Now(),
		},
		SignKey: r.signKey,
	})
	if err != nil {
		return fmt.Errorf("wt.Commit: %w", err)
	}

	return nil
}

func (r *ManifestRepository) GetCommits(ctx context.Context, base string, head string) ([]*Commit, error) {
	commits := make([]*Commit, 0)

	commitComp, _, err := r.client.Repositories.CompareCommits(
		ctx,
		r.githubRepositoryConfig.Owner,
		r.githubRepositoryConfig.Repository,
		base,
		head,
	)
	if err != nil {
		return commits, fmt.Errorf("CompareCommits: %w", err)
	}

	for _, commit := range commitComp.Commits {
		commits = append(commits, &Commit{
			Hash:           *commit.SHA,
			AuthorLogin:    *commit.Author.Login,
			CommitterLogin: *commit.Committer.Login,
		})
	}

	return commits, nil
}

func (r *ManifestRepository) GetPullRequestAssignees(ctx context.Context, sourceCommits []*Commit) ([]string, error) {
	assignees := make([]string, 0)

	assigneeMap := map[string]bool{}
	for _, sourceCommit := range sourceCommits {
		assigneeMap[sourceCommit.AuthorLogin] = true
		assigneeMap[sourceCommit.CommitterLogin] = true
	}
	for assignee := range assigneeMap {
		ok, err := r.isAssignee(ctx, assignee)
		if err != nil {
			return assignees, fmt.Errorf("isAssignee: %w", err)
		}
		if ok {
			if len(assignees) == maxAssignees {
				r.logger.Warnf("capping PR assignees at %d due to GitHub limits", maxAssignees)
				break
			}
			assignees = append(assignees, assignee)
		}
	}

	sort.Strings(assignees)
	return assignees, nil
}

func (r *ManifestRepository) isAssignee(ctx context.Context, assignee string) (bool, error) {
	// Github's own user is never a valid assignee so do not bother checking
	if assignee == webFlowUser {
		return false, nil
	}
	for _, noIssueUser := range r.noIssueUsers {
		if assignee == noIssueUser {
			return false, nil
		}
	}
	r.sleep()
	ok, _, err := r.client.Issues.IsAssignee(ctx, r.githubRepositoryConfig.Owner, r.githubRepositoryConfig.Repository, assignee)
	return ok, err
}

func (r *ManifestRepository) RaisePromotion(ctx context.Context, branchName string, pr PromotionPullRequest, assingees []string) error {
	r.logger.WithField("branch", branchName).
		Debug("Pushing new branch")

	err := r.repo.PushContext(ctx, &git.PushOptions{
		Auth:       r.auth,
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", branchName, branchName))},
	})
	if err != nil {
		return fmt.Errorf("push to origin: %w", err)
	}

	_, err = r.raisePullRequest(ctx, branchName, pr, assingees)
	if err != nil {
		return err
	}

	return nil
}

func (r *ManifestRepository) raisePullRequest(ctx context.Context, branchName string, promotionPR PromotionPullRequest, assignees []string) (*github.PullRequest, error) {
	logger := r.logger.
		WithFields(logrus.Fields{
			"branch": branchName,
			"title":  promotionPR.Title,
		})

	logger.Info("Raising pull request")

	r.sleep()
	pr, _, err := r.client.PullRequests.Create(ctx, r.githubRepositoryConfig.Owner, r.githubRepositoryConfig.Repository, &github.NewPullRequest{
		Title:               &promotionPR.Title,
		Head:                &branchName,
		Base:                &r.githubRepositoryConfig.TargetBranch,
		MaintainerCanModify: github.Bool(false),
		Body:                &promotionPR.Description,
	})
	if err != nil {
		return nil, fmt.Errorf("create PR: %w", err)
	}

	logger.
		WithFields(logrus.Fields{
			"pr":     pr.GetNumber(),
			"pr_url": pr.GetHTMLURL(),
		}).
		Infof("Pull request %d raised", pr.GetNumber())

	r.sleep()
	labels := []string{prLabel}
	_, _, err = r.client.Issues.AddLabelsToIssue(ctx, r.githubRepositoryConfig.Owner, r.githubRepositoryConfig.Repository, pr.GetNumber(), labels)
	if err != nil {
		return nil, fmt.Errorf("failed to add labels to PR: %w", err)
	}

	r.sleep()
	_, _, err = r.client.Issues.AddAssignees(ctx, r.githubRepositoryConfig.Owner, r.githubRepositoryConfig.Repository, pr.GetNumber(), assignees)
	if err != nil {
		return nil, fmt.Errorf("failed to add assignees to PR: %w", err)
	}

	return pr, nil
}

func (r *ManifestRepository) sleep() {
	time.Sleep(r.sleepDuration)
}

func ReadSignKey(filename string) (*openpgp.Entity, error) {
	in, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	keys, err := openpgp.ReadArmoredKeyRing(in)
	if err != nil {
		return nil, err
	}

	if len(keys) != 1 {
		return nil, fmt.Errorf("unexpected number of GPG keys %d, expected 1", len(keys))
	}

	return keys[0], nil
}

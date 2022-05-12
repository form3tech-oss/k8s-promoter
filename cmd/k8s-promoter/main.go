package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/form3tech/k8s-promoter/internal/promoter"
	gh "github.com/google/go-github/v33/github"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"

	"github.com/form3tech/k8s-promoter/internal/git"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

var (
	ErrMissingArg = errors.New("missing CLI argument")
	ErrMissingEnv = errors.New("missing Env variable")
)

type userList []string

const (
	Timeout = 10 * time.Minute
)

func main() {
	log := logrus.NewEntry(logrus.New())

	args, err := parseArgs()
	if err != nil {
		log.Fatalf("parseArgs: %v\n", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()

	tc := oauth2.NewClient(ctx, oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: args.CloneArgs.Auth.Password},
	))

	prom, err := promoter.NewPromoter(ctx, args, log, gh.NewClient(tc), time.Second)
	if err != nil {
		log.Fatalf("manifest.New: %v", err)
	}
	err = prom.Promote(ctx, args.TargetEnv)
	if err != nil {
		log.Fatalf("promoter.Promote: %v", err)
	}
}

func parseArgs() (*promoter.Args, error) {
	ownerArg := "owner"
	repoArg := "repository"
	branchArg := "branch"

	commitRangeArg := "commit-range"
	targetArg := "target"
	gpgKeyPathArg := "gpg-key-path"

	configRepoArg := "config-repository"
	configPathArg := "config-path"

	committerNameArg := "committer-name"
	committerEmailArg := "committer-email"

	noIssueUsersArg := "no-issue-users"

	owner := flag.String(ownerArg, "form3tech", "The repository organisation")
	repo := flag.String(repoArg, "", "The name of the target repository")
	branch := flag.String(branchArg, "master", "The name of the branch you want the changes pushed into")

	commitRange := flag.String(commitRangeArg, "", "The PR commit range which introduces changes to the workloads")
	target := flag.String(targetArg, "", "The target environment to receive promoted workload")
	gpgKeyPath := flag.String(gpgKeyPathArg, "key.gpg", "Path to the GPG key used to sign commits")

	configRepo := flag.String(configRepoArg, "", "The name of the repository to fetch the config file")
	configPath := flag.String(configPathArg, "clusters.yaml", "Path to the clusters config file")

	committerName := flag.String(committerNameArg, "", "Name of user to commit as")
	committerEmail := flag.String(committerEmailArg, "", "Email of user to commit as")

	var noIssueUsers userList
	flag.Var(&noIssueUsers, noIssueUsersArg, "GitHub user(s) that should not be assigned users (comma-separated)")

	flag.Parse()

	if empty(owner) {
		return nil, argError(ownerArg)
	}
	if empty(repo) {
		return nil, argError(repoArg)
	}
	if empty(branch) {
		return nil, argError(branchArg)
	}

	if empty(commitRange) {
		return nil, argError(commitRangeArg)
	}
	if empty(target) {
		return nil, argError(targetArg)
	}
	if empty(gpgKeyPath) {
		return nil, argError(gpgKeyPathArg)
	}

	if empty(configRepo) {
		return nil, argError(configRepoArg)
	}
	if empty(configPath) {
		return nil, argError(configPathArg)
	}

	if empty(committerName) {
		return nil, argError(committerNameArg)
	}
	if empty(committerEmail) {
		return nil, argError(committerEmailArg)
	}

	auth, err := authFromEnv()
	if err != nil {
		return nil, err
	}

	cr, err := git.NewCommitRange(*commitRange)
	if err != nil {
		return nil, err
	}

	args := &promoter.Args{
		CloneArgs: &git.CloneArgs{
			Auth:    auth,
			BaseURL: "https://github.com",
			Ref:     cr.TargetRefPrefix(),
			Branch:  *branch,
			Owner:   *owner,
			Repo:    *repo,
		},

		// used for detect
		CommitRange: cr,
		TargetEnv:   *target,

		ConfigPath:       *configPath,
		GPGKeyPath:       *gpgKeyPath,
		ConfigRepository: *configRepo,

		CommitterName:  *committerName,
		CommitterEmail: *committerEmail,

		NoIssueUsers: noIssueUsers,
	}

	return args, nil
}

func authFromEnv() (*http.BasicAuth, error) {
	user, ok := os.LookupEnv("GITHUB_USER")
	if !ok {
		return nil, fmt.Errorf("GITHUB_USER: %w", ErrMissingEnv)
	}

	token, ok := os.LookupEnv("GITHUB_TOKEN")
	if !ok {
		return nil, fmt.Errorf("GITHUB_TOKEN: %w", ErrMissingEnv)
	}

	return &http.BasicAuth{
		Username: user,
		Password: token,
	}, nil
}

func empty(s *string) bool {
	if s == nil || *s == "" {
		return true
	}

	return false
}

func argError(a string) error {
	return fmt.Errorf("%w: -%s is a required argument", ErrMissingArg, a)
}

func (u *userList) String() string {
	return strings.Join(*u, ",")
}

func (u *userList) Set(value string) error {
	*u = strings.Split(value, ",")
	return nil
}

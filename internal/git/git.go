package git

import (
	"context"
	"fmt"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	gitfilesystem "github.com/go-git/go-git/v5/storage/filesystem"
)

var ErrNoRefProvided = fmt.Errorf("empty ref")

type CloneArgs struct {
	Auth    *http.BasicAuth
	BaseURL string
	Owner   string
	Repo    string
	Branch  string
	Ref     string
}

func (c *CloneArgs) RepoURL() string {
	return fmt.Sprintf("%s/%s/%s.git", c.BaseURL, c.Owner, c.Repo)
}

func Clone(ctx context.Context, args *CloneArgs) (*git.Repository, error) {
	if args.Ref == "" {
		return nil, ErrNoRefProvided
	}

	storage := gitfilesystem.NewStorage(memfs.New(), cache.NewObjectLRUDefault())
	opts := &git.CloneOptions{
		Auth:          args.Auth,
		URL:           args.RepoURL(),
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName(args.Branch),
		SingleBranch:  false,
		Depth:         0, // Deep clone the repository. See https://github.com/form3tech/k8s-promoter/issues/3
	}
	repo, err := git.CloneContext(ctx, storage, memfs.New(), opts)
	if err != nil {
		return nil, fmt.Errorf("git.CloneContext: %v: %w", opts, err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("repo.Worktree: %w", err)
	}

	revision := plumbing.Revision(args.Ref)
	hash, err := repo.ResolveRevision(revision)
	if err != nil {
		return nil, fmt.Errorf("repo.ResolveRevision: %w", err)
	}

	// Force checkout to ignore unstaged .git file that's created by go git
	// when storer is billy.Filesystem. See vendor/github.com/go-git/go-git/v5/repository.go
	err = wt.Checkout(&git.CheckoutOptions{
		Hash:  *hash,
		Force: true,
	})
	if err != nil {
		return nil, fmt.Errorf("wt.Checkout: %w", err)
	}

	return repo, nil
}

package testutils

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	gitfilesystem "github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/stretchr/testify/require"
)

func TestFakeGit(t *testing.T) {
	tRepo := setupTestRepo(t)
	owner := "form3tech"
	repoName := "k8s-promoter"

	auth := &http.BasicAuth{
		Username: "some-test",
		Password: "some-pass",
	}

	ts := setupFakeGit(t, tRepo, owner, repoName, auth)

	storage := gitfilesystem.NewStorage(memfs.New(), cache.NewObjectLRUDefault())
	opts := &git.CloneOptions{
		Auth:          auth,
		URL:           fmt.Sprintf("%s/%s/%s.git", ts.URL, owner, repoName),
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName("master"),
		SingleBranch:  false,
		Depth:         0,
	}

	repo, err := git.CloneContext(context.Background(), storage, memfs.New(), opts)
	require.NoError(t, err)
	require.NotEmpty(t, repo)

	wt, err := repo.Worktree()
	require.NoError(t, err)
	fs := wt.Filesystem
	_ = fs
}

type testRepo struct {
	r *git.Repository
	c []plumbing.Hash
}

func setupFakeGit(t *testing.T, tRepo *testRepo, owner, repoName string, auth *http.BasicAuth) *httptest.Server {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())

	f := NewFakeGitHttp(t, tRepo.r, auth)
	f.SetupRoutes(r, owner, repoName)
	return httptest.NewServer(r)
}

func setupTestRepo(t *testing.T) *testRepo {
	r, err := git.Init(gitfilesystem.NewStorage(memfs.New(), cache.NewObjectLRUDefault()), memfs.New())
	require.NoError(t, err)

	w, err := r.Worktree()
	require.NoError(t, err)

	WriteFile(t, w.Filesystem, "some.log", "content")
	err = w.AddGlob("*")
	require.NoError(t, err)
	h, err := w.Commit("commit", &git.CommitOptions{
		All: true,
		Author: &object.Signature{
			When: time.Now(),
		},
	})
	require.NoError(t, err)

	return &testRepo{
		r: r,
		c: []plumbing.Hash{h},
	}
}

package git_test

import (
	"context"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	gitint "github.com/form3tech/k8s-promoter/internal/git"
	"github.com/form3tech/k8s-promoter/internal/testutils"
	"github.com/gin-gonic/gin"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	gitfilesystem "github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/stretchr/testify/require"
)

func Test_CloneFailsWhenAuthIsInvalid(t *testing.T) {
	tRepo := setupTestRepo(t)
	owner := "form3tech"
	repoName := "k8s-promoter"

	serverAuth := &http.BasicAuth{
		Username: "username",
		Password: "password",
	}
	clientAuth := &http.BasicAuth{
		Username: "username",
		Password: "wrong-password",
	}

	ts := setupFakeGit(t, tRepo, owner, repoName, serverAuth)

	repo, err := gitint.Clone(context.Background(), &gitint.CloneArgs{
		Auth:    clientAuth,
		BaseURL: ts.URL,
		Owner:   owner,
		Repo:    repoName,
		Branch:  "master",
		Ref:     tRepo.c[0].String(),
	})

	require.ErrorIs(t, err, transport.ErrAuthorizationFailed)
	require.Empty(t, repo)
}

func Test_CloneWorksWhenAuthIsValid(t *testing.T) {
	tRepo := setupTestRepo(t)
	owner := "form3tech"
	repoName := "k8s-promoter"

	auth := &http.BasicAuth{
		Username: "username",
		Password: "password",
	}

	ts := setupFakeGit(t, tRepo, owner, repoName, auth)

	repo, err := gitint.Clone(context.Background(), &gitint.CloneArgs{
		Auth:    auth,
		BaseURL: ts.URL,
		Owner:   owner,
		Repo:    repoName,
		Branch:  "master",
		Ref:     tRepo.c[0].String(),
	})

	require.NoError(t, err)
	require.NotEmpty(t, repo)

	obj, err := repo.CommitObject(tRepo.c[0])
	require.NoError(t, err)
	tree, err := obj.Tree()
	require.NoError(t, err)

	f, err := tree.File("some.log")
	require.NoError(t, err)
	r, err := f.Blob.Reader()
	require.NoError(t, err)

	bytes, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, string(bytes), "content")
}

func Test_ReturnsErrorWhenURLDoesNotExist(t *testing.T) {
	tRepo := setupTestRepo(t)
	owner := "form3tech"
	repoName := "k8s-promoter"

	auth := &http.BasicAuth{
		Username: "username",
		Password: "wrong-password",
	}

	repo, err := gitint.Clone(context.Background(), &gitint.CloneArgs{
		Auth:    auth,
		BaseURL: "http://wrong.invalid",
		Owner:   owner,
		Repo:    repoName,
		Branch:  "master",
		Ref:     tRepo.c[0].String(),
	})

	require.Error(t, err)
	require.Empty(t, repo)
}

func Test_ReturnsErrorWhenBranchDoesNotExist(t *testing.T) {
	tRepo := setupTestRepo(t)
	owner := "form3tech"
	repoName := "k8s-promoter"

	auth := &http.BasicAuth{
		Username: "username",
		Password: "wrong-password",
	}

	ts := setupFakeGit(t, tRepo, owner, repoName, auth)

	repo, err := gitint.Clone(context.Background(), &gitint.CloneArgs{
		Auth:    auth,
		BaseURL: ts.URL,
		Owner:   owner,
		Repo:    repoName,
		Branch:  "doesnt-exist",
		Ref:     tRepo.c[0].String(),
	})

	require.ErrorIs(t, err, plumbing.ErrReferenceNotFound)
	require.Empty(t, repo)
}

func Test_ReturnsErrorWhenRefDoesNotExist(t *testing.T) {
	tRepo := setupTestRepo(t)
	owner := "form3tech"
	repoName := "k8s-promoter"

	auth := &http.BasicAuth{
		Username: "username",
		Password: "wrong-password",
	}

	ts := setupFakeGit(t, tRepo, owner, repoName, auth)

	repo, err := gitint.Clone(context.Background(), &gitint.CloneArgs{
		Auth:    auth,
		BaseURL: ts.URL,
		Owner:   owner,
		Repo:    repoName,
		Branch:  "master",
		Ref:     "doesnt-exist",
	})

	require.ErrorIs(t, err, plumbing.ErrReferenceNotFound)
	require.Empty(t, repo)
}

type testRepo struct {
	r *git.Repository
	c []plumbing.Hash
}

func setupTestRepo(t *testing.T) *testRepo {
	r, err := git.Init(gitfilesystem.NewStorage(memfs.New(), cache.NewObjectLRUDefault()), memfs.New())
	require.NoError(t, err)

	w, err := r.Worktree()
	require.NoError(t, err)

	testutils.WriteFile(t, w.Filesystem, "some.log", "content")
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

//nolint:unparam
func setupFakeGit(t *testing.T, tRepo *testRepo, owner, repoName string, auth *http.BasicAuth) *httptest.Server {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())

	f := testutils.NewFakeGitHttp(t, tRepo.r, auth)
	f.SetupRoutes(r, owner, repoName)
	return httptest.NewServer(r)
}

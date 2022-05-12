package github

import (
	"os"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/form3tech/k8s-promoter/internal/testutils"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	gitfilesystem "github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/google/go-github/v33/github"
	"github.com/stretchr/testify/require"
)

const (
	testCommitterName  = "Test Committer"
	testCommitterEmail = "test@committer.com"
)

func Test_NewPromoteBranch_Removes_Untracked_Files(t *testing.T) {
	mr := setupManifestRepository(t)
	tree, err := mr.repo.Worktree()
	require.NoError(t, err)

	testutils.WriteFile(t, tree.Filesystem, "/unstaged.log", "foo")
	testutils.FileHasContents(t, tree.Filesystem, "/unstaged.log", "foo")

	_, err = mr.NewPromoteBranch()
	require.NoError(t, err)

	tree, err = mr.repo.Worktree()
	require.NoError(t, err)

	testutils.FileDoesNotExist(t, tree.Filesystem, "/unstaged.log")
}

func Test_Commits_Are_Signed(t *testing.T) {
	mr := setupManifestRepository(t)
	tree, err := mr.repo.Worktree()
	require.NoError(t, err)

	testutils.WriteFile(t, tree.Filesystem, "/manifests/foo/deployment.yaml", "foo")
	err = mr.Commit("Adding workload foo")
	require.NoError(t, err)

	testutils.AssertAllCommitsSigned(t, mr.signKey, mr.repo)
}

func Test_Commits_Are_From_Correct_User(t *testing.T) {
	mr := setupManifestRepository(t)
	tree, err := mr.repo.Worktree()
	require.NoError(t, err)

	testutils.WriteFile(t, tree.Filesystem, "/manifests/foo/deployment.yaml", "foo")
	err = mr.Commit("Adding workload foo")
	require.NoError(t, err)

	headRef, err := mr.repo.Head()
	require.NoError(t, err)

	testCommit, err := mr.repo.CommitObject(headRef.Hash())
	require.NoError(t, err)

	require.Equal(t, testCommitterName, testCommit.Author.Name)
	require.Equal(t, testCommitterName, testCommit.Committer.Name)
	require.Equal(t, testCommitterEmail, testCommit.Author.Email)
	require.Equal(t, testCommitterEmail, testCommit.Committer.Email)
}

func getSignKey(t *testing.T) *openpgp.Entity {
	in, err := os.Open("./testdata/key.gpg")
	require.NoError(t, err)

	keys, err := openpgp.ReadArmoredKeyRing(in)
	require.NoError(t, err)

	return keys[0]
}

func setupManifestRepository(t *testing.T) *ManifestRepository {
	signKey := getSignKey(t)
	r, err := git.Init(gitfilesystem.NewStorage(memfs.New(), cache.NewObjectLRUDefault()), memfs.New())
	require.NoError(t, err)

	hash := commit(t, r, "/some.log", "foo", signKey)
	mr, err := NewManifestRepository(
		WithRepository(r),
		WithGithubClient(github.NewClient(nil)),
		WithGithubRepositoryConfig(RepositoryConfig{
			TargetRef: hash.String(),
		}),
		WithSignKey(signKey),
		WithCommitter(testCommitterName, testCommitterEmail),
	)
	require.NoError(t, err)

	return mr
}

func commit(t *testing.T, r *git.Repository, path, content string, signKey *openpgp.Entity) plumbing.Hash {
	w, err := r.Worktree()
	require.NoError(t, err)

	testutils.WriteFile(t, w.Filesystem, path, content)
	err = w.AddGlob("*")
	require.NoError(t, err)

	h, err := w.Commit("commit", &git.CommitOptions{
		All: true,
		Author: &object.Signature{
			When: time.Now(),
		},
		SignKey: signKey,
	})
	require.NoError(t, err)

	return h
}

package github

import (
	"context"
	"testing"

	"github.com/form3tech/k8s-promoter/internal/testutils"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage"
	gitfilesystem "github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This test validates our assumption that we can use a local on disk repository as a remote for testing purposes.
// This exists solely for the purpose of documenting this behaviour.
func Test_GitAssumptions_LocalRepositoryOnDiskCanActAsRemoteForOurTests(t *testing.T) {
	remoteRepo, remoteURL := testutils.NewRepoOnDisk(t, true)
	localRepo := testutils.NewRepoFromURL(t, memfs.New(), remoteURL, testutils.WithInitialCommit)

	masterRef := plumbing.NewBranchReferenceName("master")

	localHead, err := localRepo.Head()
	require.NoError(t, err)
	assert.Equal(t, masterRef, localHead.Name(), "local HEAD should start at master")

	remoteHead, err := remoteRepo.Head()
	require.NoError(t, err)
	assert.Equal(t, masterRef, remoteHead.Name(), "remote HEAD should start at master")

	assert.Equal(t, remoteHead.Hash(), localHead.Hash(), "remote and local should be in sync")
}

// This test documents the `go-git` behaviour whereby changing the filesystem used for the `Storer` has an impact on the
// filesystem of the worktree.
func Test_GitAssumptions_StorageUsedForGitCloneCanImpactOnClonedRepoWorktree(t *testing.T) {
	cloneRepo := func(repoURL string, storer storage.Storer, worktreeFs billy.Filesystem) *git.Repository {
		clonedRepo, err := git.CloneContext(context.Background(), storer, worktreeFs, &git.CloneOptions{
			URL:           repoURL,
			RemoteName:    "origin",
			ReferenceName: plumbing.NewBranchReferenceName("master"),
			SingleBranch:  false,
			Depth:         0, // Deep clone the repository. See https://github.com/form3tech/k8s-promoter/issues/3
		})
		require.NoError(t, err)
		return clonedRepo
	}

	t.Run("Using an in memory billy.Filesystem for the repositories storage.Storer", func(t *testing.T) {
		var (
			originalRepo, dir    = testutils.NewRepoOnDisk(t, false, testutils.WithInitialCommit)
			dotGitFS, worktreeFS = memfs.New(), memfs.New()
		)

		_ = cloneRepo(dir, gitfilesystem.NewStorage(dotGitFS, cache.NewObjectLRUDefault()), worktreeFS)

		// As can be seen below, the cloned repo's worktree already contains .git file without any changes.
		// Go-git explicitly creates a `.git` file in the working tree when using a billy.Filesystem. It is not clear
		// why this is necessary.
		//
		// According to Git documentation, the .git file specifies the path of the "real directory" that has the repository:
		// https://git-scm.com/docs/git-config#_conditional_includes
		testutils.FileHasContents(t, worktreeFS, ".git", "gitdir: .\n")
		t.Logf("cloned repo worktree: %s\n", testutils.TreeFS(t, worktreeFS))

		// This file does not exist in the worktree of the original repo, instead we have a .git directory.
		testutils.DirExists(t, testutils.GetFS(t, originalRepo), ".git")
		t.Logf("original repo worktree: %s\n", testutils.TreeFS(t, testutils.GetFS(t, originalRepo)))
	})

	t.Run("Using memory.NewStorage for the repositories storage.Storer", func(t *testing.T) {
		var (
			_, dir        = testutils.NewRepoOnDisk(t, false, testutils.WithInitialCommit)
			_, worktreeFS = memfs.New(), memfs.New()
		)

		_ = cloneRepo(dir, memory.NewStorage(), worktreeFS)

		// As can be seen below, when using `memory.NewStorage()` which returns a struct based implementation of a `storage.Storer`
		// there is no .git file present in the cloned repo's worktree.
		testutils.FileDoesNotExist(t, worktreeFS, ".git")
		t.Logf("cloned repo worktree: %s\n", testutils.TreeFS(t, worktreeFS))
	})
}

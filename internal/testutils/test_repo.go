package testutils

// TODO: verify which of these helper functions are used.

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/form3tech/k8s-promoter/internal/filesystem"
	gitint "github.com/form3tech/k8s-promoter/internal/git"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	gitfilesystem "github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type RepoModifier func(r *git.Repository) error

func WithInitialCommit(r *git.Repository) error {
	_, err := DoCommit(r, "Initial commit")
	return err
}

func DoCommit(repo *git.Repository, title string, fns ...func(fs billy.Filesystem)) (plumbing.Hash, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return plumbing.ZeroHash, err
	}

	for _, f := range fns {
		f(wt.Filesystem)
	}

	err = wt.AddGlob(".")
	if err != nil {
		return plumbing.ZeroHash, err
	}

	commitHash, err := wt.Commit(title, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "",
			Email: "",
			When:  time.Now(),
		},
	})
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return commitHash, err
}

func NewRepo(t *testing.T, worktreeFs billy.Filesystem, mods ...RepoModifier) *git.Repository {
	repo, err := git.Init(gitfilesystem.NewStorage(memfs.New(), cache.NewObjectLRUDefault()), worktreeFs)
	require.NoError(t, err)

	for _, mod := range mods {
		require.NoError(t, mod(repo))
	}
	return repo
}

func NewRepoOnDisk(t *testing.T, bare bool, mods ...RepoModifier) (*git.Repository, string) {
	remoteRepoDir, err := ioutil.TempDir("", strings.ReplaceAll(t.Name(), string(filepath.Separator), "_"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(remoteRepoDir))
	})

	repo, err := git.PlainInit(remoteRepoDir, bare)
	require.NoError(t, err)

	t.Logf("Initialised test repository: %s\n", remoteRepoDir)

	for _, mod := range mods {
		require.NoError(t, mod(repo))
	}

	return repo, remoteRepoDir
}

func NewRepoFromURL(t *testing.T, worktreeFs billy.Filesystem, remoteURL string, mods ...RepoModifier) *git.Repository {
	repo, err := git.Init(gitfilesystem.NewStorage(memfs.New(), cache.NewObjectLRUDefault()), worktreeFs)
	require.NoError(t, err)

	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name:  "origin",
		URLs:  []string{remoteURL},
		Fetch: []config.RefSpec{"refs/heads/*:refs/remotes/origin/*"},
	})
	require.NoError(t, err)

	for _, mod := range mods {
		require.NoError(t, mod(repo))
	}

	// Push the commit to the remote repo so that we're in sync before the test starts.
	err = repo.PushContext(context.Background(), &git.PushOptions{
		RemoteName: "origin",
	})
	require.NoError(t, err)

	return repo
}

func FileDoesNotExist(t *testing.T, fs billy.Filesystem, path string) {
	_, err := fs.Open(path)
	assert.Errorf(t, err, "file `%s` should not exist", path)
}

func WriteFile(t *testing.T, fs billy.Filesystem, path, content string) {
	require.NoError(t, fs.MkdirAll(filepath.Dir(path), 0o777))
	require.NoError(t, util.WriteFile(fs, path, []byte(content), 0o777))
}

func DeleteFile(t *testing.T, fs billy.Filesystem, path string) {
	require.NoError(t, fs.Remove(path))
}

func Rename(t *testing.T, fs billy.Filesystem, old, new string) {
	require.NoError(t, fs.Rename(old, new))
}

func WriteDir(t *testing.T, fs billy.Filesystem, path string) {
	require.NoError(t, fs.MkdirAll(path, 0o777))
}

func FileHasContents(t *testing.T, fs billy.Filesystem, path, content string) {
	f, err := fs.Open(path)
	require.NoError(t, err)
	fContent, err := ioutil.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, content, string(fContent))
}

func DirExists(t *testing.T, fs billy.Filesystem, path string) {
	s, err := fs.Stat(path)
	require.NoError(t, err)
	assert.True(t, s.IsDir(), "%s is a file not a directory", path)
}

func AssertRepoWorktree(t *testing.T, repo *git.Repository, fns ...func(t *testing.T, worktree billy.Filesystem)) {
	wt, err := repo.Worktree()
	require.NoError(t, err)

	for _, f := range fns {
		f(t, wt.Filesystem)
	}
}

func GetFS(t *testing.T, repo *git.Repository) billy.Filesystem {
	worktree, err := repo.Worktree()
	require.NoError(t, err)
	require.NotNil(t, worktree)
	return worktree.Filesystem
}

func AssertFilesystemsAreEqual(t *testing.T, expectedFS, actualFS billy.Filesystem) {
	hash1, err := filesystem.DirHash(expectedFS, "/")
	require.NoError(t, err)
	hash2, err := filesystem.DirHash(actualFS, "/")
	require.NoError(t, err)
	require.Equal(t, hash1, hash2, fmt.Sprintf("expected %s\n actual %s\n", TreeFS(t, expectedFS), TreeFS(t, actualFS)))
}

func TreeFS(t *testing.T, fs billy.Filesystem) string {
	builder := &strings.Builder{}
	builder.WriteString("\n--------------------------------------")

	err := filesystem.WalkFiles(fs, "/", func(filePath string) error {
		builder.WriteString("\n")
		builder.WriteString(filePath)
		return nil
	})
	require.NoError(t, err)

	builder.WriteString("\n--------------------------------------\n")

	return builder.String()
}

func RepoWorktreeEqualIgnoringDotGitFolder(t *testing.T, repo1, repo2 *git.Repository) {
	expectedFS := memfs.New()
	err := filesystem.CopyFilesystem(GetFS(t, repo1), expectedFS, filesystem.IgnoreFilePath("/.git/"))
	require.NoError(t, err)

	AssertFilesystemsAreEqual(t, expectedFS, GetFS(t, repo2))
}

func AssertExactReferences(t *testing.T, repo *git.Repository, expectedRefs ...string) {
	refItr, err := repo.References()
	require.NoError(t, err)

	var observedRefs []string
	err = refItr.ForEach(func(reference *plumbing.Reference) error {
		observedRefs = append(observedRefs, reference.Name().String())
		return nil
	})
	require.NoError(t, err)

	sort.Strings(observedRefs)
	sort.Strings(expectedRefs)
	require.Equal(t, expectedRefs, observedRefs, "exact references not found")
}

func AllCommitsFromBranch(t *testing.T, repo *git.Repository, branch plumbing.ReferenceName) []*object.Commit {
	ref, err := repo.Reference(branch, false)
	require.NoError(t, err)

	commitItr, err := repo.Log(&git.LogOptions{
		From: ref.Hash(),
	})
	require.NoError(t, err)

	var commits []*object.Commit
	err = commitItr.ForEach(func(commit *object.Commit) error {
		commits = append(commits, commit)
		return nil
	})
	require.NoError(t, err)

	return commits
}

func AllFilesFromCommit(t *testing.T, commit *object.Commit) []*object.File {
	tree, err := commit.Tree()
	require.NoError(t, err)

	var files []*object.File
	err = tree.Files().ForEach(func(file *object.File) error {
		files = append(files, file)
		return nil
	})
	require.NoError(t, err)
	return files
}

func AssertAllCommitsSigned(t *testing.T, key *openpgp.Entity, repo *git.Repository) {
	// Taken from:
	// https://github.com/go-git/go-git/blob/e60e348f614a7272e4a51bdee8ba20f059ca4cce/worktree_commit_test.go#L154
	pks := new(bytes.Buffer)
	pkw, err := armor.Encode(pks, openpgp.PublicKeyType, nil)
	require.NoError(t, err)

	err = key.Serialize(pkw)
	require.NoError(t, err)

	err = pkw.Close()
	require.NoError(t, err)

	commitIter, err := repo.CommitObjects()
	require.NoError(t, err)

	err = commitIter.ForEach(func(commit *object.Commit) error {
		actual, err := commit.Verify(pks.String())
		require.NoError(t, err)

		require.Equal(t, actual.PrimaryKey, key.PrimaryKey)
		return nil
	})
	require.NoError(t, err)
}

type RepoChangeFunc func(*TestRepo)

func RepoWith(t *testing.T, changes ...RepoChangeFunc) *TestRepo {
	tr := NewTestRepo(t)

	for _, change := range changes {
		change(tr)
	}

	return tr
}

type TestRepo struct {
	t       *testing.T
	Repo    *git.Repository
	Commits []plumbing.Hash
}

type Content struct {
	Path    string
	Content string
}

func AddContent(contents []Content, msg string) RepoChangeFunc {
	return func(tr *TestRepo) {
		tr.Write(contents)
		tr.Commit(msg)
	}
}

func DeleteContent(paths []string, msg string) RepoChangeFunc {
	return func(tr *TestRepo) {
		for _, path := range paths {
			tr.Delete(path)
		}
		tr.Commit(msg)
	}
}

func RenameContent(from, to, msg string) RepoChangeFunc {
	return func(tr *TestRepo) {
		tr.Rename(from, to)
		tr.Commit(msg)
	}
}

func NewTestRepo(t *testing.T) *TestRepo {
	storage := gitfilesystem.NewStorage(memfs.New(), cache.NewObjectLRUDefault())
	repo, err := git.Init(storage, memfs.New())
	require.NoError(t, err)
	tr := &TestRepo{
		t:    t,
		Repo: repo,
	}
	return tr
}

func (tr *TestRepo) Write(content []Content) {
	fs := GetFS(tr.t, tr.Repo)
	for _, c := range content {
		WriteFile(tr.t, fs, c.Path, c.Content)
	}
}

func (tr *TestRepo) Delete(path string) {
	fs := GetFS(tr.t, tr.Repo)
	DeleteFile(tr.t, fs, path)
}

func (tr *TestRepo) Rename(from, to string) {
	fs := GetFS(tr.t, tr.Repo)
	err := fs.Rename(from, to)
	require.NoError(tr.t, err, "rename failed: %s -> %s", from, to)
}

func (tr *TestRepo) Commit(msg string) {
	wt, err := tr.Repo.Worktree()
	require.NoError(tr.t, err)

	err = wt.AddGlob(".")
	require.NoError(tr.t, err)

	hash, err := wt.Commit(msg, &git.CommitOptions{
		All: true,
		Author: &object.Signature{
			When: time.Now(),
		},
	})
	require.NoError(tr.t, err)

	tr.Commits = append(tr.Commits, hash)
}

func (tr *TestRepo) CommitRange() *gitint.CommitRange {
	// we'll go with standard 7 char prefixes as we don't have many commits
	prefix := 7

	first := tr.Commits[0]
	last := tr.Commits[len(tr.Commits)-1]

	return &gitint.CommitRange{
		FromPrefix: first.String()[:prefix],
		ToPrefix:   last.String()[:prefix],
	}
}

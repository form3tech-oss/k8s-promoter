package filesystem_test

import (
	"errors"
	"github.com/form3tech/k8s-promoter/internal/filesystem"
	"testing"

	"github.com/form3tech/k8s-promoter/internal/testutils"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ReplaceNonExistentSourceDirectory(t *testing.T) {
	fs := memfs.New()
	testutils.WriteFile(t, fs, "/target/file", "content")

	err := filesystem.Replace(fs, "/src/", "/target/")

	require.True(t, errors.Is(err, filesystem.ErrSourceDirNotExists))
}

func Test_ReplaceNonExistentTargetDirectory(t *testing.T) {
	fs := memfs.New()
	testutils.WriteFile(t, fs, "/src/file", "content")

	err := filesystem.Replace(fs, "/src/", "/target/")

	require.NoError(t, err)
	testutils.FileHasContents(t, fs, "/src/file", "content")
	testutils.FileHasContents(t, fs, "/target/file", "content")
}

func Test_ReplaceToEmptyTarget(t *testing.T) {
	fs := memfs.New()

	testutils.WriteFile(t, fs, "/src/file", "content")
	testutils.WriteDir(t, fs, "/target/")

	err := filesystem.Replace(fs, "/src/", "/target/")
	require.NoError(t, err)

	testutils.FileHasContents(t, fs, "/src/file", "content")
	testutils.FileHasContents(t, fs, "/target/file", "content")
}

func Test_ReplaceToNonEmptyTargetWithSameFile(t *testing.T) {
	fs := memfs.New()

	testutils.WriteFile(t, fs, "/src/file", "new-content")
	testutils.WriteFile(t, fs, "/target/file", "old-content")

	err := filesystem.Replace(fs, "/src/", "/target/")
	require.NoError(t, err)

	testutils.FileHasContents(t, fs, "/src/file", "new-content")
	testutils.FileHasContents(t, fs, "/target/file", "new-content")
}

func Test_ReplaceToNonEmptyTargetWithNewFile(t *testing.T) {
	fs := memfs.New()

	testutils.WriteFile(t, fs, "/src/file", "content")
	testutils.WriteFile(t, fs, "/target/anotherFile", "another-file-content")

	err := filesystem.Replace(fs, "/src/", "/target/")
	require.NoError(t, err)

	testutils.FileHasContents(t, fs, "/src/file", "content")
	testutils.FileDoesNotExist(t, fs, "/target/anotherFile")
	testutils.FileHasContents(t, fs, "/target/file", "content")
}

func Test_ReplaceToNonEmptyTargetWithSubDirectories(t *testing.T) {
	fs := memfs.New()

	testutils.WriteFile(t, fs, "/src/file", "content1")
	testutils.WriteFile(t, fs, "/src/sub1/file", "content2")
	testutils.WriteFile(t, fs, "/src/sub2/file", "content3")
	testutils.WriteDir(t, fs, "/target/")

	err := filesystem.Replace(fs, "/src/", "/target/")
	require.NoError(t, err)

	testutils.FileHasContents(t, fs, "/src/file", "content1")
	testutils.FileHasContents(t, fs, "/target/file", "content1")
	testutils.FileHasContents(t, fs, "/src/sub1/file", "content2")
	testutils.FileHasContents(t, fs, "/target/sub1/file", "content2")
	testutils.FileHasContents(t, fs, "/src/sub2/file", "content3")
	testutils.FileHasContents(t, fs, "/target/sub2/file", "content3")
}

func Test_ReplaceToNonEmptyWithSymbolicLink(t *testing.T) {
	fs := memfs.New()

	testutils.WriteFile(t, fs, "/src/file", "content")
	require.NoError(t, fs.Symlink("/src/file", "/src/sym"))
	testutils.WriteDir(t, fs, "/target/")

	err := filesystem.Replace(fs, "/src/", "/target/")
	require.NoError(t, err)

	testutils.FileHasContents(t, fs, "/src/file", "content")
	target, err := fs.Readlink("/src/sym")
	require.NoError(t, err)
	assert.Equal(t, "/src/file", target)
}

func TestCopyFilesystem(t *testing.T) {
	t.Run("EmptyFS_NoExcludes", func(t *testing.T) {
		sourceFS, targetFS := memfs.New(), memfs.New()
		err := filesystem.CopyFilesystem(sourceFS, targetFS)
		require.NoError(t, err)
		testutils.AssertFilesystemsAreEqual(t, sourceFS, targetFS)
	})

	t.Run("NonEmptyFS_NoExcludes", func(t *testing.T) {
		sourceFS, targetFS := memfs.New(), memfs.New()
		testutils.WriteFile(t, sourceFS, "/foo/bar", "baz")

		err := filesystem.CopyFilesystem(sourceFS, targetFS)
		require.NoError(t, err)
		testutils.AssertFilesystemsAreEqual(t, sourceFS, targetFS)
	})

	t.Run("EmptyFS_Excludes", func(t *testing.T) {
		sourceFS, targetFS := memfs.New(), memfs.New()
		err := filesystem.CopyFilesystem(sourceFS, targetFS, filesystem.IgnoreFilePath("/foo"))
		require.NoError(t, err)
		testutils.AssertFilesystemsAreEqual(t, sourceFS, targetFS)
	})

	t.Run("NoEmptyFS_Excludes", func(t *testing.T) {
		sourceFS, targetFS := memfs.New(), memfs.New()
		testutils.WriteFile(t, sourceFS, "/foo/bar", "baz")
		testutils.WriteFile(t, sourceFS, "/xyz/abc", "123")

		err := filesystem.CopyFilesystem(sourceFS, targetFS, filesystem.IgnoreFilePath("/foo"))
		require.NoError(t, err)

		expectedFS := memfs.New()
		testutils.WriteFile(t, expectedFS, "/xyz/abc", "123")
		testutils.AssertFilesystemsAreEqual(t, expectedFS, targetFS)
	})

	t.Run("Ignore path matches file", func(t *testing.T) {
		sourceFS, targetFS := memfs.New(), memfs.New()
		testutils.WriteFile(t, sourceFS, ".git", "gitdir: .")
		testutils.WriteFile(t, sourceFS, "/foo/bar", "baz")

		err := filesystem.CopyFilesystem(sourceFS, targetFS, filesystem.IgnoreFilePath("/.git"))
		require.NoError(t, err)

		expectedFS := memfs.New()
		testutils.WriteFile(t, expectedFS, "/foo/bar", "baz")
		testutils.AssertFilesystemsAreEqual(t, expectedFS, targetFS)
	})
}

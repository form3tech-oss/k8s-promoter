package filesystem

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"golang.org/x/mod/sumdb/dirhash"
)

var (
	ErrSourceDirNotExists = errors.New("source dir does not exist")
	ErrSourceDirEmpty     = errors.New("source dir has no manifests")
)

func Replace(fs billy.Filesystem, srcDir, targetDir string) error {
	src, err := fs.Stat(srcDir)
	if err != nil {
		return fmt.Errorf("source dir '%s' does not exist: %w", srcDir, ErrSourceDirNotExists)
	}
	if !src.IsDir() {
		return errors.New("source is not a directory")
	}
	filesToCopy, err := recursiveFilesInDir(fs, srcDir)
	if err != nil {
		return fmt.Errorf("list files in source dir: %w", err)
	}
	if len(filesToCopy) == 0 {
		return ErrSourceDirEmpty
	}

	err = util.RemoveAll(fs, targetDir)
	if err != nil {
		return err
	}

	return WalkFiles(fs, srcDir, func(file string) error {
		pathFromSrcAsBase := strings.TrimPrefix(file, srcDir)
		newFilePath := filepath.Join(targetDir, pathFromSrcAsBase)

		err = fs.MkdirAll(filepath.Dir(newFilePath), 0o777)
		if err != nil {
			return fmt.Errorf("create subdir in target dir: %w", err)
		}

		sourceFile, err := fs.Open(file)
		if err != nil {
			return fmt.Errorf("open source file %s: %w", sourceFile, err)
		}

		targetFile, err := fs.Create(newFilePath)
		if err != nil {
			return fmt.Errorf("create file in target dir: %w", err)
		}

		_, err = io.Copy(targetFile, sourceFile)
		if err != nil {
			return fmt.Errorf("copy source file to target: %w", err)
		}

		if err = targetFile.Close(); err != nil {
			return fmt.Errorf("targetFile.Close: %w", err)
		}

		if err = sourceFile.Close(); err != nil {
			return fmt.Errorf("sourceFile.Close(): %w", err)
		}

		return nil
	})
}

type fileWalker func(filePath string) error

func WalkFiles(fs billy.Filesystem, base string, walker fileWalker) error {
	files, err := fs.ReadDir(base)
	if err != nil {
		return fmt.Errorf("walk directory %s: %w", base, err)
	}
	for _, f := range files {
		filePath := filepath.Join(base, f.Name())
		if f.IsDir() {
			err = WalkFiles(fs, filePath, walker)
			if err != nil {
				return fmt.Errorf("walking %s subdir %s: %w", base, f.Name(), err)
			}
		} else {
			err = walker(filePath)
			if err != nil {
				return fmt.Errorf("walking %s file %s: %w", base, f.Name(), err)
			}
		}
	}
	return nil
}

// DirHash hashes the files in `dir` using relative file path for its comparison.
func DirHash(fs billy.Filesystem, dir string) (string, error) {
	files, err := recursiveFilesInDir(fs, dir)
	if err != nil {
		return "", err
	}

	// Trim directory prefix from file path as hashing logic uses full path.
	var relativeFilePaths []string
	for _, file := range files {
		relativeFilePaths = append(relativeFilePaths, strings.TrimPrefix(file, dir))
	}

	return dirhash.Hash1(relativeFilePaths, func(s string) (io.ReadCloser, error) {
		// Reapply directory
		return fs.Open(filepath.Join(dir, s))
	})
}

// recursiveFilesInDir returns all files in the file tree under dir.
func recursiveFilesInDir(fs billy.Filesystem, dir string) ([]string, error) {
	var files []string
	err := WalkFiles(fs, dir, func(filePath string) error {
		files = append(files, filePath)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk directory %s: %w", dir, err)
	}
	return files, nil
}

// dirsInDir returns the list of all directories in `dir`.
func DirsInDir(fs billy.Filesystem, dir string) ([]string, error) {
	files, err := fs.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir files: %w", err)
	}

	var dirNames []string
	for _, f := range files {
		if f.IsDir() {
			dirNames = append(dirNames, f.Name())
		}
	}
	return dirNames, nil
}

func IgnoreFilePath(pathPrefix string) func(string, os.FileInfo) bool {
	return func(filePath string, _ os.FileInfo) bool {
		return !strings.HasPrefix(filePath, pathPrefix)
	}
}

// CopyFilesystem copies all files from the source filesystem to the target filesystem.
// Where a file already exists on `target`, it will be overwritten.
func CopyFilesystem(src, target billy.Filesystem, predicates ...func(filePath string, fileInfo os.FileInfo) bool) error {
	return WalkFiles(src, "/", func(filePath string) error {
		stat, err := src.Stat(filePath)
		if err != nil {
			return err
		}

		for _, p := range predicates {
			if ok := p(filePath, stat); !ok {
				return nil
			}
		}

		srcFile, err := src.Open(filePath)
		if err != nil {
			return err
		}
		targetFile, err := target.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, stat.Mode())
		if err != nil {
			return err
		}
		_, err = io.Copy(targetFile, srcFile)
		if err != nil {
			return err
		}

		if err = targetFile.Close(); err != nil {
			return fmt.Errorf("targetFile.Close: %w", err)
		}

		if err = srcFile.Close(); err != nil {
			return fmt.Errorf("srcFile.Close(): %w", err)
		}

		return nil
	})
}

//go:build mage
// +build mage

package main

import (
	"path"

	"github.com/magefile/mage/sh"
	"github.com/princjef/mageutil/bintool"
)

const lintVersion = "1.46.1"

const project = "k8s-promoter"

var linter = bintool.Must(bintool.New(
	"golangci-lint{{.BinExt}}",
	lintVersion,
	"https://github.com/golangci/golangci-lint/releases/download/v{{.Version}}/golangci-lint-{{.Version}}-{{.GOOS}}-{{.GOARCH}}{{.ArchiveExt}}",
	bintool.WithFolder("tools"),
))

func Build() error {
	return sh.RunV("go", "build", "-o", path.Join("bin", project), path.Join("cmd", project, "main.go"))
}

func Lint() error {
	if err := linter.Ensure(); err != nil {
		return err
	}

	return linter.Command(`run`).Run()
}

func LintFix() error {
	if err := linter.Ensure(); err != nil {
		return err
	}

	return linter.Command(`run --fix`).Run()
}

func Test() error {
	return sh.RunV("go", "test", "--timeout", "10m", "-v", "-count", "1", "./...")
}

func Vendor() error {
	return sh.RunV("go", "mod", "vendor")
}

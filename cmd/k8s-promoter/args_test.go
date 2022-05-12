package main

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/form3tech/k8s-promoter/internal/git"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setArgs(args map[string]string) {
	os.Args = []string{"promote"}
	for k, v := range args {
		os.Args = append(os.Args, []string{k, v}...)
	}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
}

func setAuth(t *testing.T, username, token string) {
	require.NoError(t, os.Setenv("GITHUB_USER", username))
	require.NoError(t, os.Setenv("GITHUB_TOKEN", token))
}

func getDefaultArgs() map[string]string {
	return map[string]string{
		"-repository":        "repository",
		"-branch":            "branch",
		"-owner":             "owner",
		"-commit-range":      "start...end",
		"-target":            "target-env",
		"-gpg-key-path":      "path/to/key.gpg",
		"-config-repository": "config-repository",
		"-config-path":       "path/to/config.yaml",
		"-committer-name":    "Test Committer",
		"-committer-email":   "test@committer.com",
		"-no-issue-users":    "some-bot-1,some-bot-2",
	}
}

func Test_arguments_are_set(t *testing.T) {
	cliArgs := getDefaultArgs()
	setArgs(cliArgs)
	setAuth(t, "username", "token")

	args, err := parseArgs()
	require.NoError(t, err)
	assert.Equal(t, "target-env", args.TargetEnv)

	assert.Equal(t, &git.CommitRange{
		FromPrefix: "start",
		ToPrefix:   "end",
	}, args.CommitRange)

	assert.Equal(t, "owner", args.CloneArgs.Owner)
	assert.Equal(t, "repository", args.CloneArgs.Repo)
	assert.Equal(t, "branch", args.CloneArgs.Branch)
	assert.Equal(t, "username", args.CloneArgs.Auth.Username)
	assert.Equal(t, "token", args.CloneArgs.Auth.Password)
	assert.Equal(t, "path/to/key.gpg", args.GPGKeyPath)

	assert.Equal(t, "path/to/config.yaml", args.ConfigPath)
	assert.Equal(t, "config-repository", args.ConfigRepository)

	assert.Equal(t, []string{"some-bot-1", "some-bot-2"}, args.NoIssueUsers)
}

func Test_empty_branch_default(t *testing.T) {
	cliArgs := getDefaultArgs()
	delete(cliArgs, "-branch")

	setArgs(cliArgs)
	setAuth(t, "username", "token")

	args, err := parseArgs()
	require.NoError(t, err)
	assert.Equal(t, "master", args.CloneArgs.Branch)
}

func Test_empty_owner_defaults(t *testing.T) {
	cliArgs := getDefaultArgs()
	delete(cliArgs, "-owner")

	setArgs(cliArgs)
	setAuth(t, "username", "token")

	args, err := parseArgs()
	require.NoError(t, err)
	assert.Equal(t, "form3tech", args.CloneArgs.Owner)
}

func Test_empty_config_default(t *testing.T) {
	cliArgs := getDefaultArgs()
	delete(cliArgs, "-config-path")

	setArgs(cliArgs)
	setAuth(t, "username", "token")

	args, err := parseArgs()
	require.NoError(t, err)
	assert.Equal(t, "clusters.yaml", args.ConfigPath)
}

func Test_empty_gpg_key_default(t *testing.T) {
	cliArgs := getDefaultArgs()
	delete(cliArgs, "-gpg-key-path")

	setArgs(cliArgs)
	setAuth(t, "username", "token")

	args, err := parseArgs()
	require.NoError(t, err)
	assert.Equal(t, "key.gpg", args.GPGKeyPath)
}

func Test_empty_no_issue_users_default(t *testing.T) {
	cliArgs := getDefaultArgs()
	delete(cliArgs, "-no-issue-users")

	setArgs(cliArgs)
	setAuth(t, "username", "token")

	args, err := parseArgs()
	require.NoError(t, err)
	assert.Equal(t, 0, len(args.NoIssueUsers))
}

func Test_empty_required_field(t *testing.T) {
	tests := map[string]struct {
		flagName string
	}{
		"empty target": {
			flagName: "-target",
		},
		"empty repo": {
			flagName: "-repository",
		},
		"empty commit range": {
			flagName: "-commit-range",
		},
		"empty config repository": {
			flagName: "-config-repository",
		},
		"empty committer name": {
			flagName: "-committer-name",
		},
		"empty committer email": {
			flagName: "-committer-email",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cliArgs := getDefaultArgs()
			delete(cliArgs, tt.flagName)

			setArgs(cliArgs)
			setAuth(t, "username", "token")

			_, err := parseArgs()

			message := fmt.Sprintf("missing CLI argument: %s is a required argument", tt.flagName)
			require.EqualError(t, err, message)
		})
	}
}

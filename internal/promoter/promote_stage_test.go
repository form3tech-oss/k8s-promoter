package promoter_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/form3tech/k8s-promoter/internal/clusterconf"
	"github.com/form3tech/k8s-promoter/internal/environment"
	"github.com/form3tech/k8s-promoter/internal/promoter"
	"gopkg.in/yaml.v2"

	git2 "github.com/form3tech/k8s-promoter/internal/git"
	"github.com/form3tech/k8s-promoter/internal/testutils"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	gitfilesystem "github.com/go-git/go-git/v5/storage/filesystem"
	gh "github.com/google/go-github/v33/github"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	oldContent            string = "old-content"
	newContent            string = "new-content"
	kustomizationTemplate        = `---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:{{range $element := .}}
  - ./{{$element}}{{end}}
`
	user0     = "test-user-0"
	user1     = "test-user-1"
	user2     = "test-user-2"
	user3     = "test-user-3"
	user4     = "test-user-4"
	buildUser = "github-builduser-form3"
)

type CommitRange struct {
	Start string
	End   string
}

type SourceCommit struct {
	Hash           string
	AuthorLogin    string
	CommitterLogin string
}

func (c CommitRange) Validate() error {
	if c.Start == "" || c.End == "" {
		return fmt.Errorf("Commit ranges are not set: start: %s, end: %s", c.Start, c.End)
	}
	return nil
}

type PromoteStage struct {
	t           *testing.T
	logBuffer   *bytes.Buffer
	args        promoter.Args
	commitRange CommitRange
	err         error

	repository *git.Repository
	githubFake *testutils.GithubFake

	pr         gh.PullRequest
	prCommit   *object.Commit
	parentHash plumbing.Hash

	expSourceCommits []SourceCommit
}

func PromoteTest(t *testing.T) (*PromoteStage, *PromoteStage, *PromoteStage) {
	stage := &PromoteStage{
		t: t,
		args: promoter.Args{
			GPGKeyPath:       "../github/testdata/key.gpg",
			ConfigRepository: "infrastructure-k8s-admin",
			CloneArgs: &git2.CloneArgs{
				Owner:  "form3tech",
				Repo:   "k8s-promoter",
				Branch: "master",
				Auth: &http.BasicAuth{
					Username: "some-test",
					Password: "some-pass",
				},
			},
		},
	}
	return stage, stage, stage
}

// Given.
func (s *PromoteStage) a_fake_github_server() *PromoteStage {
	githubFake := testutils.NewGithubFake(
		s.t,
		testutils.WithOrgAndRepo(s.args.CloneArgs.Owner, s.args.CloneArgs.Repo),
		testutils.WithGitFakeForRepo(s.repository, s.args.CloneArgs.Auth),
		testutils.WithRepoAssignees("test-user-1", "test-user-2", "test-user-3", "test-user-4"),
		testutils.WithConfigRepo(s.args.ConfigRepository),
	)
	githubFake.StartServer().InitClient()

	_, err := s.repository.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{githubFake.RepoURL()},
	})
	require.NoError(s.t, err)

	s.githubFake = githubFake
	return s
}

func (s *PromoteStage) a_repository() *PromoteStage {
	repo, err := git.Init(gitfilesystem.NewStorage(memfs.New(), cache.NewObjectLRUDefault()), memfs.New())
	require.NoError(s.t, err)

	w, err := repo.Worktree()
	require.NoError(s.t, err)

	testutils.WriteFile(s.t, w.Filesystem, ".keep", "foo")

	prTemplate, err := os.ReadFile("./testdata/pr_template.md")
	require.NoError(s.t, err)
	testutils.WriteFile(s.t, w.Filesystem, ".github/PULL_REQUEST_TEMPLATE/master.md", string(prTemplate))

	s.repository = repo
	s.CommitChange("Initial commit", user0, user0, false, false)
	return s
}

func (s *PromoteStage) deleted_source_manifests_for_the_workload(workload string) *PromoteStage {
	w, err := s.repository.Worktree()
	require.NoError(s.t, err)
	testutils.DeleteFile(s.t, w.Filesystem, path(fmt.Sprintf("/manifests/%s/file", workload)))
	s.CommitChange(fmt.Sprintf("Remove source manifests for workload %s", workload), user3, user4, false, true)

	return s
}

func (s *PromoteStage) a_clusters_configuration_file() *PromoteStage {
	clusters := allClusters()
	clustersYAML, err := toYAML(clusters)
	require.NoError(s.t, err)

	s.githubFake.SetContent("clusters.yaml", clustersYAML)
	s.args.ConfigPath = "clusters.yaml"
	return s
}

func (s *PromoteStage) a_clusters_configuration_file_with_new_dev_cluster() *PromoteStage {
	clusters := allClusters()
	clusters = append(clusters, cluster("development", "dev1", "cloud1"))

	clustersYAML, err := toYAML(clusters)
	require.NoError(s.t, err)

	s.githubFake.SetContent("clusters.yaml", clustersYAML)
	s.args.ConfigPath = "clusters.yaml"
	return s
}

func (s *PromoteStage) a_clusters_configuration_file_with_new_test_clusters() *PromoteStage {
	clusters := allClusters()
	clusters = append(clusters, cluster("test", "new-test-cluster-1", "cloud1"))
	clusters = append(clusters, cluster("test", "new-test-cluster-2", "cloud2"))

	clustersYAML, err := toYAML(clusters)
	require.NoError(s.t, err)

	s.githubFake.SetContent("clusters.yaml", clustersYAML)

	s.args.ConfigPath = "clusters.yaml"
	return s
}

func (s *PromoteStage) a_clusters_configuration_file_with_new_prod_clusters() *PromoteStage {
	clusters := allClusters()
	clusters = append(clusters, cluster("production", "new-prd", "cloud1"))

	clustersYAML, err := toYAML(clusters)
	require.NoError(s.t, err)

	s.githubFake.SetContent("clusters.yaml", clustersYAML)
	s.args.ConfigPath = "clusters.yaml"
	return s
}

func (s *PromoteStage) a_clusters_file_with_only_dev_clusters() *PromoteStage {
	clusters := []clusterconf.Cluster{
		cluster("development", "dev2", "cloud1"),
		cluster("development", "dev3", "cloud1"),
		cluster("development", "dev4", "cloud2"),
	}

	clustersYAML, err := toYAML(clusters)
	require.NoError(s.t, err)

	s.githubFake.SetContent("dev_clusters.yaml", clustersYAML)
	s.args.ConfigPath = "dev_clusters.yaml"
	return s
}

func (s *PromoteStage) an_cloud1_only_config_file_for_foo() *PromoteStage {
	return s.a_workload_config_file_for_foo(
		`version: "v0.1"
configType: Workload
metadata:
  name: foo
  description: "A workload with no exclusions"
spec:
  path: /flux/manifests/foo
  exclusions: # These apply to all versions
  - key: "cloud"
    operator: "NotEqual"
    value: "cloud1"
`)
}

func (s *PromoteStage) a_workload_config_file_for_foo(content string) *PromoteStage {
	wt, err := s.repository.Worktree()
	require.NoError(s.t, err)

	fs := wt.Filesystem
	testutils.WriteFile(s.t, fs, path("/manifests/foo/workload.yaml"), content)

	s.CommitChange("Workload config file", user0, user0, false, false)
	return s
}

func (s *PromoteStage) a_promoted_manifest_for_the_workload(workload, env, cluster, cloud, content string) *PromoteStage {
	clusterKustomizationBase := path(fmt.Sprintf("/promoted/%s/%s/%s", env, cluster, cloud))
	path := path(fmt.Sprintf("/promoted/%s/%s/%s/%s/file", env, cluster, cloud, workload))

	wt, err := s.repository.Worktree()
	require.NoError(s.t, err)

	fs := wt.Filesystem
	testutils.WriteFile(s.t, fs, path, content)

	// kustomization
	kustomizationPath := filepath.Join(clusterKustomizationBase, "kustomization.yaml")
	f, err := fs.Create(kustomizationPath)
	require.NoError(s.t, err)
	s.t.Cleanup(func() {
		f.Close()
	})

	tmpl, err := template.New("kustomization").Parse(kustomizationTemplate)
	require.NoError(s.t, err)

	err = tmpl.Execute(f, []string{workload})
	require.NoError(s.t, err)
	// kustomization end

	return s
}

func (s *PromoteStage) old_source_manifests_for_the_workload(workload string) *PromoteStage {
	// Track old manifests as a source-commit only if they're made within our commit range
	return s.source_manifests_for_the_workload(workload, oldContent, user1, user2, s.commitRange.Start != "")
}

func (s *PromoteStage) new_source_manifests_for_the_workload(workload string) *PromoteStage {
	// Always track new manifests as a source-commit
	return s.source_manifests_for_the_workload(workload, newContent, user2, user3, true)
}

func (s *PromoteStage) with_config_for_the_workload(workload string) *PromoteStage {
	wt, err := s.repository.Worktree()
	require.NoError(s.t, err)

	for _, cluster := range allClusters() {
		testutils.WriteFile(s.t, wt.Filesystem, fmt.Sprintf("%s/%s-config.yaml", cluster.ConfigFolder(), workload), "test content")
		testutils.WriteFile(s.t, wt.Filesystem, fmt.Sprintf("%s/kustomization.yaml", cluster.ConfigFolder()), "test kustomization content")
	}

	s.CommitChange(fmt.Sprintf("Adding config for workload %s", workload), user2, user2, false, false)

	return s
}

func (s *PromoteStage) source_manifests_for_the_workload(workload string, content string, author string, committer string, trackCommit bool) *PromoteStage {
	wt, err := s.repository.Worktree()
	require.NoError(s.t, err)

	testutils.WriteFile(s.t, wt.Filesystem, path(fmt.Sprintf("/manifests/%s/file", workload)), content)
	s.CommitChange(fmt.Sprintf("Adding workload %s source manifests", workload), author, committer, false, trackCommit)

	return s
}

func (s *PromoteStage) source_manifest_renamed_in_the_workload(workload string) *PromoteStage {
	wt, err := s.repository.Worktree()
	require.NoError(s.t, err)

	testutils.Rename(s.t, wt.Filesystem,
		filepath.Join("flux", "manifests", workload, "file"),
		filepath.Join("flux", "manifests", workload, "file2"),
	)

	s.CommitChange("Commit rename", user1, user4, false, true)

	return s
}

func (s *PromoteStage) manifest_for_workload_foo_is_renamed_to_bar() *PromoteStage {
	wt, err := s.repository.Worktree()
	require.NoError(s.t, err)

	testutils.Rename(s.t, wt.Filesystem,
		filepath.Join("flux", "manifests", "foo"),
		filepath.Join("flux", "manifests", "bar"),
	)

	s.CommitChange("Commit rename", user1, user4, false, true)

	return s
}

func (s *PromoteStage) non_workload_update(path, content string) *PromoteStage {
	wt, err := s.repository.Worktree()
	require.NoError(s.t, err)

	testutils.WriteFile(s.t, wt.Filesystem, path, content)
	// We shouldn't really be considering this as a source commit, but the implementation is currently unable to
	// filter this out from the source manifest changes. So the tests have to expect this to match.
	s.CommitChange(fmt.Sprintf("Updating %s", path), user0, user0, false, true)

	return s
}

// takes the last commit as start for the commit range.
func (s *PromoteStage) commit_range_start() *PromoteStage {
	s.commitRange.Start = s.parentHash.String()
	s.githubFake.SetBaseCommit(s.parentHash.String())

	return s
}

// takes the last commit as end for commit range.
func (s *PromoteStage) commit_range_end() *PromoteStage {
	s.commitRange.End = s.parentHash.String()

	return s
}

func (s *PromoteStage) empty_commit_range() *PromoteStage {
	s.commitRange.Start = s.parentHash.String()
	s.commitRange.End = s.parentHash.String()
	s.githubFake.SetBaseCommit(s.parentHash.String())
	return s
}

func (s *PromoteStage) old_dev_manifests_for_the_workload_foo() *PromoteStage {
	return s.dev_manifests_for_the_workload_foo(oldContent, false)
}

func (s *PromoteStage) old_dev_manifests_for_the_workload_bar() *PromoteStage {
	s.a_promoted_manifest_for_the_workload("bar", "development", "dev2", "cloud1", oldContent)
	s.a_promoted_manifest_for_the_workload("bar", "development", "dev3", "cloud1", oldContent)
	s.a_promoted_manifest_for_the_workload("bar", "development", "dev4", "cloud2", oldContent)

	s.CommitChange("Commit initial manifests", buildUser, buildUser, false, false)

	return s
}

func (s *PromoteStage) new_dev_manifests_for_the_workload_foo() *PromoteStage {
	return s.dev_manifests_for_the_workload_foo(newContent, true)
}

func (s *PromoteStage) dev_manifests_for_the_workload_foo(content string, writeSourceCommits bool) *PromoteStage {
	s.a_promoted_manifest_for_the_workload("foo", "development", "dev2", "cloud1", content)
	s.a_promoted_manifest_for_the_workload("foo", "development", "dev3", "cloud1", content)
	s.a_promoted_manifest_for_the_workload("foo", "development", "dev4", "cloud2", content)

	s.CommitChange("Commit initial manifests", buildUser, buildUser, writeSourceCommits, false)

	return s
}

func (s *PromoteStage) old_test_manifests_for_the_workload_foo() *PromoteStage {
	return s.test_manifests_for_the_workload_foo(oldContent, false)
}

func (s *PromoteStage) old_prod_manifests_for_the_workload_foo() *PromoteStage {
	s.a_promoted_manifest_for_the_workload("foo", "production", "prod1", "cloud1", oldContent)
	s.a_promoted_manifest_for_the_workload("foo", "production", "prod2", "cloud1", oldContent)
	s.a_promoted_manifest_for_the_workload("foo", "production", "prod3", "cloud2", oldContent)

	s.CommitChange("Commit initial manifests", buildUser, buildUser, true, false)

	return s
}

func (s *PromoteStage) new_test_manifests_for_the_workload_foo() *PromoteStage {
	return s.test_manifests_for_the_workload_foo(newContent, true)
}

func (s *PromoteStage) test_manifests_for_the_workload_foo(content string, writeSourceCommits bool) *PromoteStage {
	s.a_promoted_manifest_for_the_workload("foo", "test", "test1", "cloud1", content)
	s.a_promoted_manifest_for_the_workload("foo", "test", "test2", "cloud1", content)
	s.a_promoted_manifest_for_the_workload("foo", "test", "test3", "cloud2", content)

	s.CommitChange("Commit initial manifests", buildUser, buildUser, writeSourceCommits, false)

	return s
}

func (s *PromoteStage) a_file_with_content(path, content string) *PromoteStage {
	wt, err := s.repository.Worktree()
	require.NoError(s.t, err)

	testutils.WriteFile(s.t, wt.Filesystem, path, content)
	s.CommitChange("inconsistent workload write", user0, user0, false, false)

	return s
}

// helper to commit changes in working tree
// important that parentHash is set appropiately for every commit.
func (s *PromoteStage) CommitChange(msg string, author string, committer string, writeSourceCommits bool, trackAsSourceCommit bool) {
	if writeSourceCommits {
		// Commit messages could contain \r\n or \n. Use \r\n here as that is the harder case
		msg = msg + "\r\n"
		for _, sourceCommit := range s.expSourceCommits {
			msg = fmt.Sprintf(
				"%s\nSource-commit: %s A:%s C:%s",
				msg,
				sourceCommit.Hash,
				sourceCommit.AuthorLogin,
				sourceCommit.CommitterLogin,
			)
		}
		// Random trailing tag
		msg = msg + "\r\nFoo: bar"
	}

	wt, err := s.repository.Worktree()
	require.NoError(s.t, err)

	err = wt.AddGlob("*")
	require.NoError(s.t, err)

	hash, err := wt.Commit(msg, &git.CommitOptions{
		All: true,
		Author: &object.Signature{
			When: time.Now(),
		},
	})
	require.NoError(s.t, err)

	s.parentHash = hash
	require.NoError(s.t, err)

	if s.commitRange.Start != "" {
		s.githubFake.AddCommit(hash.String(), author, committer)
	}

	if trackAsSourceCommit {
		s.expSourceCommits = append(s.expSourceCommits, SourceCommit{
			Hash:           hash.String(),
			AuthorLogin:    author,
			CommitterLogin: committer,
		})
	}
}

// When

func (s *PromoteStage) promote() *PromoteStage {
	return s
}

func (s *PromoteStage) with_env(env environment.Env) *PromoteStage {
	s.args.TargetEnv = string(env)
	return s
}

func (s *PromoteStage) with_no_issue_users(users ...string) *PromoteStage {
	s.args.NoIssueUsers = users
	return s
}

func (s *PromoteStage) is_called() *PromoteStage {
	require.NoError(s.t, s.commitRange.Validate())

	// construct the commit range
	s.args.CloneArgs.Ref = s.commitRange.End

	cr, err := git2.NewCommitRange(fmt.Sprintf("%s...%s", s.commitRange.Start[:7], s.commitRange.End[:7]))
	require.NoError(s.t, err)

	s.args.CommitRange = cr

	s.args.CommitterName = "Test Committer"
	s.args.CommitterEmail = "test@committer.com"

	s.args.CloneArgs.Branch = "master"
	s.args.CloneArgs.BaseURL = s.githubFake.URL()

	buf := &bytes.Buffer{}
	logger := logrus.New()
	logger.SetOutput(buf)
	log := logrus.NewEntry(logger)
	s.logBuffer = buf

	prom, err := promoter.NewPromoter(context.Background(), &s.args, log, s.githubFake.Client, 0)
	require.NoError(s.t, err)

	s.err = prom.Promote(context.Background(), s.args.TargetEnv)
	return s
}

// Then

func (s *PromoteStage) promote_succeeds() *PromoteStage {
	require.NoError(s.t, s.err)
	return s
}

func (s *PromoteStage) a_message_is_logged(msg string, level logrus.Level) *PromoteStage {
	logs := s.logBuffer.String()

	rxStr := fmt.Sprintf("level=%s .*%s.*", level.String(), regexp.QuoteMeta(msg))
	rx := regexp.MustCompile(rxStr)
	require.Regexp(s.t, rx, logs)

	return s
}

func (s *PromoteStage) the_remote_repository_is_not_updated_with_new_branch() *PromoteStage {
	return s.the_remote_repository_is_updated_with_new_branches(0)
}

func (s *PromoteStage) the_remote_repository_is_updated_with_new_branch() *PromoteStage {
	return s.the_remote_repository_is_updated_with_new_branches(1)
}

func (s *PromoteStage) the_remote_repository_is_updated_with_2_new_branches() *PromoteStage {
	return s.the_remote_repository_is_updated_with_new_branches(2)
}

func (s *PromoteStage) the_remote_repository_is_updated_with_3_new_branches() *PromoteStage {
	return s.the_remote_repository_is_updated_with_new_branches(3)
}

func (s *PromoteStage) the_remote_repository_is_updated_with_new_branches(num int) *PromoteStage {
	itr, err := s.repository.References()
	require.NoError(s.t, err)

	var references []*plumbing.Reference
	err = itr.ForEach(func(reference *plumbing.Reference) error {
		if reference.Name() != "HEAD" && reference.Name() != "refs/heads/master" && reference.Name().IsBranch() {
			references = append(references, reference)
		}
		return nil
	})
	require.NoError(s.t, err)

	require.Len(s.t, references, num)
	return s
}

func (s *PromoteStage) a_PR_for(workload string, env environment.Env, clusters ...string) *PromoteStage {
	keywords := []string{workload, string(env)}
	keywords = append(keywords, clusters...)

	pr := s.githubFake.FindPRWhereTitleContains(keywords...)
	require.NotZero(s.t, pr)
	require.NotNil(s.t, pr.Body)

	s.pr = pr
	return s
}

func (s *PromoteStage) the_number_of_raised_PRs_equals(n int) *PromoteStage {
	assert.Equal(s.t, n, len(s.githubFake.CreatedPullRequests), "the number of raised PRs doesn't match the expectation")
	return s
}

func (s *PromoteStage) has_branch() *PromoteStage {
	assert.NotNil(s.t, s.pr.Head)
	assert.Equal(s.t, "master", *s.pr.Base.Ref)
	return s
}

func (s *PromoteStage) with_one_commit() *PromoteStage {
	reference, err := s.repository.Reference(plumbing.NewBranchReferenceName(*s.pr.Head.Ref), false)
	require.NoError(s.t, err)

	commitHash := reference.Hash()
	commit, err := s.repository.CommitObject(commitHash)
	require.NoError(s.t, err)
	s.prCommit = commit

	tree, _ := commit.Tree()
	parentCommit, _ := commit.Parents().Next()
	parentTree, _ := parentCommit.Tree()

	changes, _ := parentTree.Diff(tree)
	_ = changes

	repoParentCommit, err := s.repository.CommitObject(commit.ParentHashes[0])
	require.NoError(s.t, err)
	_ = repoParentCommit

	require.Len(s.t, commit.ParentHashes, 1)
	assert.Equal(s.t, commit.ParentHashes[0], s.parentHash)
	return s
}

func (s *PromoteStage) with_source_commit() *PromoteStage {
	assert.Equal(s.t, 1, len(get_source_commits(s.prCommit.Message)))
	return s.with_source_commits()
}

func (s *PromoteStage) with_source_commits() *PromoteStage {
	require.NotNil(s.t, s.prCommit, "prCommit not set")
	assert.Equalf(
		s.t,
		s.expSourceCommits,
		get_source_commits(s.prCommit.Message),
		"Source commit(s) unexpected in commit message:\n %s",
		s.prCommit.Message,
	)
	return s
}

func get_source_commits(commitMessage string) []SourceCommit {
	regex := regexp.MustCompile(`Source-commit: (.*) A:(.*) C:(.*)`)
	matches := regex.FindAllStringSubmatch(commitMessage, -1)

	var sourceCommits []SourceCommit
	for _, match := range matches {
		sourceCommits = append(sourceCommits, SourceCommit{
			Hash:           match[1],
			AuthorLogin:    match[2],
			CommitterLogin: match[3],
		})
	}

	return sourceCommits
}

func (s *PromoteStage) has_labels(labels ...string) *PromoteStage {
	for _, l := range labels {
		res := s.githubFake.FindPRLabel(s.pr.GetNumber(), l)
		require.NotZero(s.t, res, "cannot find label '%s' on PR #%d", l, s.pr.GetNumber())
		require.Equal(s.t, l, *res.Name)
	}
	return s
}

func (s *PromoteStage) has_assignees(assignees ...string) *PromoteStage {
	res := s.githubFake.FindPRAssignees(s.pr.GetNumber())
	require.NotZero(s.t, res, "cannot find assignees on PR #%d", s.pr.GetNumber())
	require.Equal(s.t, assignees, res)
	return s
}

func (s *PromoteStage) has_no_assignees() *PromoteStage {
	res := s.githubFake.FindPRAssignees(s.pr.GetNumber())
	require.Empty(s.t, res, "found assignees on PR #%d", s.pr.GetNumber())
	return s
}

func (s *PromoteStage) that_contains_updated_foo_manifests_for_cluster(clusterManifestDir string) *PromoteStage {
	s.that_contains_updated_foo_manifests_for_clusters(clusterManifestDir)
	return s
}

func (s *PromoteStage) that_contains_updated_foo_manifests_for_clusters(clusterManifestDirs ...string) *PromoteStage {
	return s.that_contains_updated_workload_manifests_for_clusters("foo", clusterManifestDirs...)
}

func (s *PromoteStage) that_contains_updated_bar_manifests_for_clusters(clusterManifestDirs ...string) *PromoteStage {
	return s.that_contains_updated_workload_manifests_for_clusters("bar", clusterManifestDirs...)
}

func (s *PromoteStage) that_contains_bar_manifests_for_clusters(clusterManifestDirs ...string) *PromoteStage {
	return s.that_contains_workload_manifests_for_clusters("bar", clusterManifestDirs...)
}

func (s *PromoteStage) that_contains_updated_workload_manifests_for_clusters(workload string, clusterManifestDirs ...string) *PromoteStage {
	tree, err := s.prCommit.Tree()
	require.NoError(s.t, err)

	for _, d := range clusterManifestDirs {
		d = path(d)
		d = strings.TrimLeft(d, "/")
		workloadDir := filepath.Join(d, workload)

		filep := filepath.Join(workloadDir, "file")
		file, err := tree.File(filep)
		require.NoError(s.t, err, "%s", filep)
		contents, err := file.Contents()
		require.NoError(s.t, err)
		assert.Equal(s.t, newContent, contents)
	}

	return s
}

func (s *PromoteStage) that_contains_workload_manifests_for_clusters(workload string, clusterManifestDirs ...string) *PromoteStage {
	tree, err := s.prCommit.Tree()
	require.NoError(s.t, err)

	for _, d := range clusterManifestDirs {
		d = path(d)
		d = strings.TrimLeft(d, "/")
		workloadDir := filepath.Join(d, workload)

		filep := filepath.Join(workloadDir, "file")
		file, err := tree.File(filep)
		require.NoError(s.t, err, "%s", filep)
		contents, err := file.Contents()
		require.NoError(s.t, err)
		assert.Equal(s.t, oldContent, contents)
	}

	return s
}

func (s *PromoteStage) that_contains_renamed_manifest_only_in_the_workload(clusterManifestDirs ...string) *PromoteStage {
	tree, err := s.prCommit.Tree()
	require.NoError(s.t, err)

	for _, d := range clusterManifestDirs {
		d = path(d)
		d = strings.TrimLeft(d, "/")
		workloadDir := filepath.Join(d, "foo")

		filep := filepath.Join(workloadDir, "file2")
		file, err := tree.File(filep)
		require.NoError(s.t, err, "%s", filep)
		contents, err := file.Contents()
		require.NoError(s.t, err)
		assert.Equal(s.t, oldContent, contents)
	}

	return s
}

func (s *PromoteStage) that_contains_renamed_workload(clusterManifestDirs ...string) *PromoteStage {
	tree, err := s.prCommit.Tree()
	require.NoError(s.t, err)

	for _, d := range clusterManifestDirs {
		d = path(d)
		d = strings.TrimLeft(d, "/")
		workloadDir := filepath.Join(d, "bar")

		filep := filepath.Join(workloadDir, "file")
		file, err := tree.File(filep)
		require.NoError(s.t, err, "%s", filep)
		contents, err := file.Contents()
		require.NoError(s.t, err)
		assert.Equal(s.t, oldContent, contents)
	}

	return s
}

func path(p string) string {
	return filepath.Join("/flux", p)
}

func (s *PromoteStage) that_deletes_manifests(workload string, clusterManifestDirs ...string) *PromoteStage {
	tree, err := s.prCommit.Tree()
	require.NoError(s.t, err)

	for _, d := range clusterManifestDirs {
		d = path(d)
		d = strings.TrimLeft(d, "/")
		workloadDir := filepath.Join(d, workload)

		filep := filepath.Join(workloadDir, "file")
		_, err := tree.File(filep)
		require.ErrorIs(s.t, err, object.ErrFileNotFound)
	}

	return s
}

func (s *PromoteStage) that_contains_changes_only_for_directory(dir string) *PromoteStage {
	return s.that_contains_foo_changes_only_for_directories(dir)
}

func (s *PromoteStage) that_contains_foo_changes_only_for_directories(dirs ...string) *PromoteStage {
	return s.that_contains_workload_changes_only_for_directories("foo", dirs...)
}

func (s *PromoteStage) that_contains_bar_changes_only_for_directories(dirs ...string) *PromoteStage {
	return s.that_contains_workload_changes_only_for_directories("bar", dirs...)
}

func (s *PromoteStage) that_contains_workload_changes_only_for_directories(workload string, dirs ...string) *PromoteStage {
	tree, err := s.prCommit.Tree()
	require.NoError(s.t, err)

	var allowedPaths []*regexp.Regexp
	for _, d := range dirs {
		d = path(d)
		d = strings.TrimLeft(d, "/")
		workloadDir := filepath.Join(d, workload)
		workloadDirRegexp := regexp.MustCompile(fmt.Sprintf("^%s", regexp.QuoteMeta(workloadDir)))
		allowedPaths = append(allowedPaths, workloadDirRegexp)

		configDir := filepath.Join(d, fmt.Sprintf("%s-config.yaml", workload))
		configDirRegexp := regexp.MustCompile(fmt.Sprintf("^%s", regexp.QuoteMeta(configDir)))
		allowedPaths = append(allowedPaths, configDirRegexp)

		kustomizationPath := filepath.Join(d, "kustomization.yaml")
		kustomizationPathRegexp := regexp.MustCompile(fmt.Sprintf("^%s$", regexp.QuoteMeta(kustomizationPath)))
		allowedPaths = append(allowedPaths, kustomizationPathRegexp)
	}

	parentCommit, err := s.prCommit.Parents().Next()
	// head, err := s.remoteRepo.Head()
	require.NoError(s.t, err)

	// commit, err := s.remoteRepo.CommitObject(head.Hash())
	// require.NoError(s.t, err)

	initialCommitTree, err := parentCommit.Tree()
	tree.Files()
	require.NoError(s.t, err)

	changes, err := initialCommitTree.Diff(tree)
	require.NoError(s.t, err)

	for _, change := range changes {
		allowed := false
		for _, r := range allowedPaths {
			if r.MatchString(change.To.Name) {
				allowed = true
				break
			}
		}

		assert.True(s.t, allowed, "Files outside of allowed paths included in commit: %s\nAllowed paths: %s", change.To.Name, allowedPaths)
	}

	return s
}

func (s *PromoteStage) that_deletes_kustomization_for_workload(workload string, clusterManifestDirs ...string) *PromoteStage {
	tree, err := s.prCommit.Tree()
	require.NoError(s.t, err)

	for _, d := range clusterManifestDirs {
		d = path(d)
		d = strings.TrimLeft(d, "/")
		kustomizationFile := filepath.Join(d, workload, "kustomization.yaml")

		_, err := tree.File(kustomizationFile)
		require.ErrorIs(s.t, err, object.ErrFileNotFound)
	}

	return s
}

func (s *PromoteStage) that_has_kustomization_for_workloads(clusterManifestDir string, workloads ...string) *PromoteStage {
	tree, err := s.prCommit.Tree()
	require.NoError(s.t, err)

	clusterManifestDir = path(clusterManifestDir)
	clusterManifestDir = strings.TrimLeft(clusterManifestDir, "/")
	filep := filepath.Join(clusterManifestDir, "kustomization.yaml")
	file, err := tree.File(filep)
	require.NoError(s.t, err, "%s", filep)
	contents, err := file.Contents()
	require.NoError(s.t, err)

	kustomizationTemplate := `---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:{{range $element := .}}
  - ./{{$element}}{{end}}
`

	tmpl, err := template.New("kustomization").Parse(kustomizationTemplate)
	require.NoError(s.t, err)

	kustomizationContent := &strings.Builder{}
	err = tmpl.Execute(kustomizationContent, workloads)
	require.NoError(s.t, err)

	assert.Equal(s.t, kustomizationContent.String(), contents)
	return s
}

func allClusters() clusterconf.Clusters {
	return clusterconf.Clusters{
		cluster("development", "dev2", "cloud1"),
		cluster("development", "dev3", "cloud1"),
		cluster("development", "dev4", "cloud2"),
		cluster("test", "test1", "cloud1"),
		cluster("test", "test2", "cloud1"),
		cluster("test", "test3", "cloud2"),
		cluster("production", "prod1", "cloud1"),
		cluster("production", "prod2", "cloud1"),
		cluster("production", "prod3", "cloud1"),
	}
}

func cluster(env, name, cloud string) clusterconf.Cluster {
	return clusterconf.Cluster{
		Version:    "v0.1",
		ConfigType: "Cluster",
		Metadata: clusterconf.ClusterMetadata{
			Name: fmt.Sprintf("%s-%s", name, cloud),
			Labels: clusterconf.Labels{
				"environment": env,
				"cloud":       cloud,
			},
		},
		Spec: clusterconf.ClusterSpec{
			ManifestFolder: path(fmt.Sprintf("/promoted/%s/%s/%s", env, name, cloud)),
			ConfigFolder:   path(fmt.Sprintf("/config/%s/%s/%s", env, name, cloud)),
		},
	}
}

func toYAML(clusters clusterconf.Clusters) (string, error) {
	builder := strings.Builder{}
	count := len(clusters) - 1

	for i := range clusters {
		// to fix G601: Implicit memory aliasing in for loop. (gosec)
		dump, err := yaml.Marshal(&clusters[i])
		if err != nil {
			return "", err
		}

		builder.WriteString(string(dump))

		// skip separator for last cluster
		if i < count {
			builder.WriteString("---\n")
		}
	}

	return builder.String(), nil
}

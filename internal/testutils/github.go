package testutils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	http2 "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/gin-gonic/gin"
	"github.com/go-git/go-git/v5"
	"github.com/google/go-github/v33/github"
	"github.com/stretchr/testify/require"
)

type GithubFake struct {
	Client                  *github.Client
	server                  *httptest.Server
	CreatedPullRequests     []github.PullRequest
	CreateLabelRequests     []AddLabelRequest
	CreateAssigneesRequests []AddAssigneesRequest
	t                       *testing.T
	r                       *gin.Engine
	orgName                 string
	repoName                string
	adminRepoName           string
	repoAssignees           []string
	gitFake                 *FakeGit
	baseCommit              *commitFake
	commits                 []*commitFake
	content                 map[string]string
}

type commitFake struct {
	hash           string
	authorLogin    string
	committerLogin string
}

type AddLabelRequest struct {
	IssueNumber int
	Labels      []github.Label
}

type AddAssigneesRequest struct {
	IssueNumber int
	Assignees   []string
}

type GitHubFakeOption func(gh *GithubFake)

func NewGithubFake(t *testing.T, opts ...GitHubFakeOption) *GithubFake {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())

	f := &GithubFake{t: t, r: r}
	for _, op := range opts {
		op(f)
	}

	f.SetupRoutes(f.r)
	if f.gitFake != nil {
		f.gitFake.SetupRoutes(r, f.orgName, f.repoName)
	}

	f.content = make(map[string]string)

	return f
}

func WithOrgAndRepo(org, repo string) GitHubFakeOption {
	return func(gh *GithubFake) {
		gh.orgName = org
		gh.repoName = repo
	}
}

func WithConfigRepo(repo string) GitHubFakeOption {
	return func(gh *GithubFake) {
		gh.adminRepoName = repo
	}
}

func WithGitFakeForRepo(gitRepo *git.Repository, auth *http2.BasicAuth) GitHubFakeOption {
	return func(gh *GithubFake) {
		gh.gitFake = NewFakeGitHttp(gh.t, gitRepo, auth)
	}
}

func WithRepoAssignees(assignees ...string) GitHubFakeOption {
	return func(gh *GithubFake) {
		gh.repoAssignees = assignees
	}
}

func (f *GithubFake) StartServer() *GithubFake {
	f.server = httptest.NewServer(f.r)
	return f
}

func (f *GithubFake) SetContent(path, content string) {
	f.content[path] = content
}

func (f *GithubFake) URL() string {
	return f.server.URL
}

func (f *GithubFake) RepoURL() string {
	return fmt.Sprintf("%s/%s/%s.git", f.URL(), f.orgName, f.repoName)
}

func (f *GithubFake) SetBaseCommit(hash string) *GithubFake {
	f.baseCommit = &commitFake{hash: hash}
	return f
}

func (f *GithubFake) AddCommit(hash string, authorLogin string, committerLogin string) *GithubFake {
	f.commits = append(f.commits,
		&commitFake{
			hash:           hash,
			authorLogin:    authorLogin,
			committerLogin: committerLogin,
		},
	)
	return f
}

func (f *GithubFake) SetupRoutes(r *gin.Engine) {
	commits := fmt.Sprintf("/api/v3/repos/%s/%s/compare/:base_head", f.orgName, f.repoName)
	r.GET(commits, f.handleCommitComparison)

	contents := fmt.Sprintf("/api/v3/repos/%s/%s/contents/:path", f.orgName, f.adminRepoName)
	r.GET(contents, f.handleContents)

	isAssignee := fmt.Sprintf("/api/v3/repos/%s/%s/assignees/:assignee", f.orgName, f.repoName)
	r.GET(isAssignee, f.handleIsAssignee)

	pulls := fmt.Sprintf("/api/v3/repos/%s/%s/pulls", f.orgName, f.repoName)
	r.POST(pulls, f.handleCreatePullRequest)

	issues := fmt.Sprintf("/api/v3/repos/%s/%s/issues/:number/labels", f.orgName, f.repoName)
	r.POST(issues, f.handleAddIssueLabels)

	createAssignees := fmt.Sprintf("/api/v3/repos/%s/%s/issues/:number/assignees", f.orgName, f.repoName)
	r.POST(createAssignees, f.handleAddAssignees)
}

func (f *GithubFake) InitClient() *GithubFake {
	var err error
	f.Client, err = github.NewEnterpriseClient(f.server.URL, f.server.URL, http.DefaultClient)
	require.NoError(f.t, err)
	return f
}

func (f *GithubFake) handleCommitComparison(c *gin.Context) {
	baseHead, ok := c.Params.Get("base_head")
	require.True(f.t, ok, "missing base_head to commit comparison")
	require.NotNil(f.t, f.baseCommit, "no base commit set")

	// real GitHub API returs empty commit list when you compare commit to itself (58fc87a9ce9a...58fc87a9ce9a).
	// Without the below change, out fake would panic.

	parts := strings.Split(baseHead, "...")
	if parts[0] != parts[1] {
		require.NotZero(f.t, len(f.commits), "no commits added")

		headCommit := f.commits[len(f.commits)-1]
		require.Equal(f.t, fmt.Sprintf("%s...%s", f.baseCommit.hash[:7], headCommit.hash[:7]), baseHead)
	}

	var commits []*github.RepositoryCommit
	for _, commit := range f.commits {
		commits = append(
			commits,
			&github.RepositoryCommit{
				SHA:       &commit.hash,
				Author:    &github.User{Login: &commit.authorLogin},
				Committer: &github.User{Login: &commit.committerLogin},
			},
		)
	}

	comp := github.CommitsComparison{Commits: commits}

	res, err := json.Marshal(comp)
	require.NoError(f.t, err)

	_, err = c.Writer.Write(res)
	require.NoError(f.t, err)
}

func (f *GithubFake) handleIsAssignee(c *gin.Context) {
	assignee, ok := c.Params.Get("assignee")
	require.True(f.t, ok, "missing assignee")

	for _, repoAssignee := range f.repoAssignees {
		if assignee == repoAssignee {
			c.Writer.WriteHeader(http.StatusNoContent)
			return
		}
	}

	c.Writer.WriteHeader(http.StatusNotFound)
}

func (f *GithubFake) handleCreatePullRequest(c *gin.Context) {
	var newPR github.NewPullRequest

	err := json.NewDecoder(c.Request.Body).Decode(&newPR)
	require.NoError(f.t, err)

	prNumber := len(f.CreatedPullRequests) + 1

	resPR := github.PullRequest{
		Title: newPR.Title,
		Body:  newPR.Body,
		Base: &github.PullRequestBranch{
			Ref: newPR.Base,
		},
		Head: &github.PullRequestBranch{
			Ref: newPR.Head,
		},
		Number: &prNumber,
	}
	res, err := json.Marshal(resPR)
	require.NoError(f.t, err)

	_, err = c.Writer.Write(res)
	require.NoError(f.t, err)

	f.CreatedPullRequests = append(f.CreatedPullRequests, resPR)
}

func (f *GithubFake) handleContents(c *gin.Context) {
	require.NotEmpty(f.t, f.content)

	path, ok := c.Params.Get("path")
	require.True(f.t, ok, "missing path")

	content, ok := f.content[path]
	require.True(f.t, ok, "missing path in content map")

	rType := "file"
	enc := "base64"
	encoded := base64.StdEncoding.EncodeToString([]byte(content))

	resp := &github.RepositoryContent{
		Type:     &rType,
		Encoding: &enc,
		Name:     &path,
		Path:     &path,
		Content:  &encoded,
	}

	res, err := json.Marshal(resp)
	require.NoError(f.t, err)

	_, err = c.Writer.Write(res)
	require.NoError(f.t, err)
}

func (f *GithubFake) handleAddIssueLabels(c *gin.Context) {
	issueNumberParam, ok := c.Params.Get("number")
	require.True(f.t, ok, "missing PR number while trying to create new PR label")

	issueNumber, err := strconv.Atoi(issueNumberParam)
	require.NoError(f.t, err)

	var newLabels []string
	err = json.NewDecoder(c.Request.Body).Decode(&newLabels)
	require.NoError(f.t, err)

	var resLabels []github.Label
	for i := range newLabels {
		resLabels = append(resLabels, github.Label{
			Name: &newLabels[i],
		})
	}

	res, err := json.Marshal(resLabels)
	require.NoError(f.t, err)

	_, err = c.Writer.Write(res)
	require.NoError(f.t, err)

	addLabelRequest := AddLabelRequest{
		IssueNumber: issueNumber,
		Labels:      resLabels,
	}
	f.CreateLabelRequests = append(f.CreateLabelRequests, addLabelRequest)
}

func (f *GithubFake) handleAddAssignees(c *gin.Context) {
	issueNumberParam, ok := c.Params.Get("number")
	require.True(f.t, ok, "missing PR number while trying to create new PR assignees")

	issueNumber, err := strconv.ParseInt(issueNumberParam, 10, 64)
	require.NoError(f.t, err)

	reqBody := struct {
		Assignees []string `json:"assignees"`
	}{}
	err = json.NewDecoder(c.Request.Body).Decode(&reqBody)
	require.NoError(f.t, err)
	reqAssignees := reqBody.Assignees

	for _, assignee := range reqAssignees {
		found := false
		for _, repoAssignee := range f.repoAssignees {
			if assignee == repoAssignee {
				found = true
				break
			}
		}
		if !found {
			c.Writer.WriteHeader(http.StatusNotFound)
			return
		}
	}

	var resAssignees []*github.User
	for i := range reqAssignees {
		resAssignees = append(resAssignees, &github.User{
			Name: &reqAssignees[i],
		})
	}

	res, err := json.Marshal(github.Issue{ID: &issueNumber, Assignees: resAssignees})
	require.NoError(f.t, err)

	_, err = c.Writer.Write(res)
	require.NoError(f.t, err)

	addAssigneesRequest := AddAssigneesRequest{
		IssueNumber: int(issueNumber),
		Assignees:   reqAssignees,
	}
	f.CreateAssigneesRequests = append(f.CreateAssigneesRequests, addAssigneesRequest)
}

func (f *GithubFake) FindPRWhereTitleContains(keywords ...string) github.PullRequest {
	for _, newPR := range f.CreatedPullRequests {
		contains := true
		for _, keyword := range keywords {
			if !strings.Contains(*newPR.Title, keyword) && !strings.Contains(*newPR.Body, keyword) {
				contains = false
			}
		}

		if contains {
			return newPR
		}
	}

	f.t.Errorf("cannot find PR where title contains keywords: %s", strings.Join(keywords, ", "))
	return github.PullRequest{}
}

func (f *GithubFake) FindPRLabel(number int, label string) github.Label {
	for _, r := range f.CreateLabelRequests {
		if r.IssueNumber != number {
			continue
		}

		for _, l := range r.Labels {
			if *l.Name == label {
				return l
			}
		}
	}

	return github.Label{}
}

func (f *GithubFake) FindPRAssignees(number int) []string {
	var assignees []string

	for _, r := range f.CreateAssigneesRequests {
		if r.IssueNumber == number {
			return r.Assignees
		}
	}

	f.t.Errorf("cannot find assignees for PR %d", number)
	return assignees
}

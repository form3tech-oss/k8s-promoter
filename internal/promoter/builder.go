package promoter

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"

	"github.com/form3tech/k8s-promoter/internal/environment"
	"github.com/form3tech/k8s-promoter/internal/github"
	promotion "github.com/form3tech/k8s-promoter/internal/promotion"
	"github.com/go-git/go-billy/v5"
	"github.com/sirupsen/logrus"
)

const (
	PRTemplatePath = ".github/PULL_REQUEST_TEMPLATE/master.md"

	PromotionsSectionTemplate = `{{- template "origin" . -}}
{{- template "description" .Description -}}

{{- define "origin" -}}
### Origin{{ "\n\n" }}
{{- if .NewClusterPromotion -}}
This promotes all workloads to newly detected cluster(s).{{ "\n" }}
:warning: **Please update config files as needed** :warning:
{{ "\n\n" }}
{{- else -}}
{{- template "source-list" .SourceManifestListView -}}
{{- end -}}
{{- template "table" .TableView -}}
{{- end -}}

{{- define "source-list" -}}
{{- if len . | empty -}}
This promotion is based on unknown source manifest changes.{{ "\n\n" }}
{{- else -}}
{{- "This promotion is based on the following source manifest changes(s):" -}}{{ "\n" }}
{{- range . -}}* {{ . -}}{{ "\n" }}{{- end -}}
{{ "\n" }}
{{- end -}}
{{- end -}}

{{- define "description" -}}
### Description{{ "\n\n" }}
{{- . -}}
{{- end -}}

{{- define "table" -}}
{{- if .NotEmpty -}}
Promotions:{{ "\n" }}
{{- range . -}}{{- template "table-row" . -}}{{- end -}}
{{- end -}}
{{- end -}}

{{- define "table-row" -}}
|{{- range . -}}{{- . -}}|{{- end -}}
{{ "\n" }}
{{- end -}}
`
)

// PullRequestBuilder is responsible for building description, title and commit message for promotion pull request.
type PullRequestBuilder struct {
	env                 environment.Env
	logger              *logrus.Entry
	pullRequestTemplate []byte
	promotionsTemplate  *template.Template
}

type descriptionView struct {
	SourceManifestListView sourceManifestListView
	Description            string
	TableView              tableView
	NewClusterPromotion    bool
}

type sourceManifestListView []string

type tableView [][]string

func (v tableView) NotEmpty() bool {
	return len(v) > 0
}

func NewPullRequestBuilder(fs billy.Filesystem, log *logrus.Entry, env environment.Env) (*PullRequestBuilder, error) {
	f, err := fs.Open(PRTemplatePath)

	if os.IsNotExist(err) {
		return nil, fmt.Errorf("ManifestRepository is missing PR template. A template is expected at %s", PRTemplatePath)
	}

	if err != nil {
		return nil, err
	}

	defer func() {
		if err := f.Close(); err != nil {
			log.WithError(err).Error("f.Close")
		}
	}()

	pullRequestTemplate, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	promotionsTemplate, err := template.New("pr").Funcs(template.FuncMap{
		"empty": func(i int) bool {
			return i == 0
		},
	}).Parse(PromotionsSectionTemplate)
	if err != nil {
		return nil, err
	}

	return &PullRequestBuilder{
		env:                 env,
		pullRequestTemplate: pullRequestTemplate,
		logger:              log.WithField("module", "PullRequestBuilder"),
		promotionsTemplate:  promotionsTemplate,
	}, nil
}

func (p *PullRequestBuilder) Build(promotions promotion.Results, commits []*github.Commit, kind promotion.Kind) github.PromotionPullRequest {
	return github.PromotionPullRequest{
		CommitMessage: p.buildCommitMessage(promotions, commits),
		Description:   p.buildDescription(commits, promotions, kind),
		Title:         p.buildTitle(promotions),
	}
}

func (b *PullRequestBuilder) buildDescription(sourceCommits []*github.Commit, promotions promotion.Results, promotionType promotion.Kind) string {
	buf := bytes.NewBuffer(nil)

	err := b.promotionsTemplate.Execute(
		buf,
		descriptionView{
			SourceManifestListView: buildSourceManifestListView(sourceCommits),
			Description:            string(b.pullRequestTemplate),
			TableView:              buildTableView(promotions, promotionType),
			NewClusterPromotion:    promotionType == promotion.NewCluster,
		},
	)
	if err != nil {
		panic(err)
	}

	return buf.String()
}

func (p *PullRequestBuilder) buildCommitMessage(promotions promotion.Results, sourceCommits []*github.Commit) string {
	prTitle := p.buildTitle(promotions)

	commitMsg := prTitle
	if len(sourceCommits) > 0 {
		commitMsg = commitMsg + "\n"
		for _, c := range sourceCommits {
			commitMsg = fmt.Sprintf("%s\nSource-commit: %s A:%s C:%s",
				commitMsg,
				c.Hash,
				c.AuthorLogin,
				c.CommitterLogin,
			)
		}
	}
	return commitMsg
}

func (p *PullRequestBuilder) buildTitle(promotions promotion.Results) string {
	title := fmt.Sprintf("Promote %s to %s", strings.Join(promotions.WorkloadNames(), ", "), p.env)

	if p.env != environment.Development {
		title += fmt.Sprintf(" (%s)", strings.Join(promotions.ClusterNames(), ", "))
	}

	return title
}

func buildSourceManifestListView(commits []*github.Commit) sourceManifestListView {
	var list sourceManifestListView
	for _, commit := range commits {
		item := fmt.Sprintf("%s - @%s", commit.Hash, commit.AuthorLogin)
		if commit.AuthorLogin != commit.CommitterLogin && commit.CommitterLogin != "web-flow" {
			item += fmt.Sprintf(" @%s", commit.CommitterLogin)
		}
		list = append(list, item)
	}
	return list
}

func buildTableView(promotions promotion.Results, kind promotion.Kind) tableView {
	var table tableView
	if len(promotions) == 0 {
		return table
	}

	var (
		headers = []string{""}
		divider = []string{"-"}
	)
	for _, name := range promotions.WorkloadNames() {
		headers = append(headers, name)
		divider = append(divider, "-")
	}
	table = append(table, headers)
	table = append(table, divider)

	for _, clusterName := range promotions.ClusterNames() {
		var row []string
		clusterCell := clusterName
		if kind == promotion.NewCluster {
			clusterCell += " (new)"
		}
		row = append(row, clusterCell)

		for _, workloadName := range promotions.WorkloadNames() {
			_, exists := promotions[clusterName][workloadName]
			if !exists {
				row = append(row, "-")
			} else {
				row = append(row, ":heavy_check_mark:")
			}
		}

		table = append(table, row)
	}

	return table
}

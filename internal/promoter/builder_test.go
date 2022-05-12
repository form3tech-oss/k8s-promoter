package promoter_test

import (
	"testing"

	"github.com/form3tech/k8s-promoter/internal/detect"
	"github.com/form3tech/k8s-promoter/internal/environment"
	"github.com/form3tech/k8s-promoter/internal/github"
	"github.com/form3tech/k8s-promoter/internal/promoter"
	promotion "github.com/form3tech/k8s-promoter/internal/promotion"
	"github.com/form3tech/k8s-promoter/internal/testutils"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestPRBuilder_Description(t *testing.T) {
	tests := map[string]struct {
		commits       []*github.Commit
		promotions    promotion.Results
		promotionType promotion.Kind
		want          string
	}{
		"empty source commits and promotion results": {
			commits:       nil,
			promotions:    nil,
			promotionType: promotion.ManifestUpdate,
			want: `### Origin

This promotion is based on unknown source manifest changes.

### Description

template`,
		},
		"not empty source commits and promotion results": {
			commits: []*github.Commit{
				{
					Hash:           "b9cfd3a",
					AuthorLogin:    "login-1",
					CommitterLogin: "login-1",
				},
				{
					Hash:           "ea2720b",
					AuthorLogin:    "login-1",
					CommitterLogin: "login-2",
				},
				{
					Hash:           "814d9d0",
					AuthorLogin:    "login-1",
					CommitterLogin: "web-flow",
				},
			},
			promotions: promotion.Results{
				"dev1": {
					"foo": detect.WorkloadChange{
						W: detect.Workload{Name: "foo"},
					},
					"bar": detect.WorkloadChange{
						W: detect.Workload{Name: "bar"},
					},
				},
				"dev4": {
					"foo": detect.WorkloadChange{
						W: detect.Workload{Name: "foo"},
					},
					"bar": detect.WorkloadChange{
						W: detect.Workload{Name: "bar"},
					},
				},
			},
			promotionType: promotion.ManifestUpdate,
			want: `### Origin

This promotion is based on the following source manifest changes(s):
* b9cfd3a - @login-1
* ea2720b - @login-1 @login-2
* 814d9d0 - @login-1

Promotions:
||bar|foo|
|-|-|-|
|dev1|:heavy_check_mark:|:heavy_check_mark:|
|dev4|:heavy_check_mark:|:heavy_check_mark:|
### Description

template`,
		},
		"not empty source commits and promotion results for new cluster": {
			commits: []*github.Commit{
				{
					Hash:           "b9cfd3a",
					AuthorLogin:    "login-1",
					CommitterLogin: "login-1",
				},
				{
					Hash:           "ea2720b",
					AuthorLogin:    "login-1",
					CommitterLogin: "login-2",
				},
			},
			promotions: promotion.Results{
				"dev1": {
					"foo": detect.WorkloadChange{
						W: detect.Workload{Name: "foo"},
					},
					"bar": detect.WorkloadChange{
						W: detect.Workload{Name: "bar"},
					},
				},
				"dev4": {
					"foo": detect.WorkloadChange{
						W: detect.Workload{Name: "foo"},
					},
					"bar": detect.WorkloadChange{
						W: detect.Workload{Name: "bar"},
					},
				},
			},
			promotionType: promotion.NewCluster,
			want: `### Origin

This promotes all workloads to newly detected cluster(s).

:warning: **Please update config files as needed** :warning:


Promotions:
||bar|foo|
|-|-|-|
|dev1 (new)|:heavy_check_mark:|:heavy_check_mark:|
|dev4 (new)|:heavy_check_mark:|:heavy_check_mark:|
### Description

template`,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// given
			fs := memfs.New()
			testutils.WriteFile(t, fs, promoter.PRTemplatePath, "template")
			l := logrus.NewEntry(logrus.New())

			builder, err := promoter.NewPullRequestBuilder(fs, l, environment.Development)
			require.NoError(t, err)

			// when
			got := builder.Build(tt.promotions, tt.commits, tt.promotionType)

			// then
			require.Equal(t, tt.want, got.Description)
		})
	}
}

func Test_PRBuilder_Title(t *testing.T) {
	tests := map[string]struct {
		results   promotion.Results
		targetEnv environment.Env
		want      string
	}{
		"single workload promoted to dev": {
			results: promotion.Results{
				"dev1": {
					"foo": detect.WorkloadChange{
						W: detect.Workload{
							Name: "foo",
						},
					},
				},
			},
			targetEnv: environment.Development,
			want:      "Promote foo to development",
		},
		"many workloads promoted to dev": {
			results: promotion.Results{

				"dev1": {
					"foo": detect.WorkloadChange{
						W: detect.Workload{
							Name: "foo",
						},
					},
					"bar": detect.WorkloadChange{
						W: detect.Workload{
							Name: "bar",
						},
					},
				},
			},
			targetEnv: environment.Development,
			want:      "Promote bar, foo to development",
		},
		"single workload promoted to test": {
			results: promotion.Results{
				"dev1": {
					"foo": detect.WorkloadChange{
						W: detect.Workload{
							Name: "foo",
						},
					},
				},
			},
			targetEnv: environment.Test,
			want:      "Promote foo to test (dev1)",
		},
		"many workloads promoted to test": {
			results: promotion.Results{
				"dev1": {
					"foo": detect.WorkloadChange{
						W: detect.Workload{
							Name: "foo",
						},
					},
					"bar": detect.WorkloadChange{
						W: detect.Workload{
							Name: "bar",
						},
					},
				},
			},
			targetEnv: environment.Test,
			want:      "Promote bar, foo to test (dev1)",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			fs := memfs.New()
			testutils.WriteFile(t, fs, promoter.PRTemplatePath, "template")
			l := logrus.NewEntry(logrus.New())

			builder, err := promoter.NewPullRequestBuilder(fs, l, tt.targetEnv)
			require.NoError(t, err)

			got := builder.Build(tt.results, []*github.Commit{}, promotion.ManifestUpdate)
			require.Equal(t, tt.want, got.Title)
		})
	}
}

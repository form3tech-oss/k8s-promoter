package detect_test

import (
	"testing"

	"github.com/form3tech/k8s-promoter/internal/clusterconf"
	"github.com/form3tech/k8s-promoter/internal/detect"
	gitint "github.com/form3tech/k8s-promoter/internal/git"
	"github.com/form3tech/k8s-promoter/internal/github"
	"github.com/form3tech/k8s-promoter/internal/testutils"
	"github.com/go-git/go-git/v5"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := map[string]struct {
		gitRepo     *git.Repository
		commitRange *gitint.CommitRange
		expected    *detect.Detect
		expectedErr error
	}{
		"when repo is nil": {
			gitRepo:     nil,
			commitRange: &gitint.CommitRange{},
			expected:    nil,
			expectedErr: detect.ErrRepoNotInitialised,
		},
		"when valid repo and commit range": {
			gitRepo: &git.Repository{},
			commitRange: &gitint.CommitRange{
				FromPrefix: "992483b570a9",
				ToPrefix:   "08759b4b4d6c",
			},
			expected: &detect.Detect{
				Repo: &git.Repository{},
				CR: &gitint.CommitRange{
					FromPrefix: "992483b570a9",
					ToPrefix:   "08759b4b4d6c",
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			l := logrus.NewEntry(logrus.New())
			got, err := detect.NewDetect(tt.gitRepo, tt.commitRange, dummyWorkloadRegistry{}, l)
			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
				require.Empty(t, got)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected.Repo, got.Repo)
			require.Equal(t, tt.expected.CR, got.CR)
		})
	}
}

func TestDiff(t *testing.T) {
	tests := map[string]struct {
		TestRepo *testutils.TestRepo
		expect   []detect.WorkloadChange
		err      error
	}{
		"Adding a new workload": {
			testutils.RepoWith(t,
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload2/kustomization.yaml",
							Content: "some content",
						},
					},
					"first commit",
				),
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload1/kustomization.yaml",
							Content: "some content",
						},
					},
					"adding workload1",
				),
			),
			[]detect.WorkloadChange{
				{
					Op: detect.OperationCopy,
					W: detect.Workload{
						Name:      "workload1",
						SourceEnv: "manifests",
					},
				},
			},
			nil,
		},
		"Adding a new workload with multiple manifests": {
			testutils.RepoWith(t,
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload2/kustomization.yaml",
							Content: "some content",
						},
					},
					"first commit",
				),

				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload1/kustomization.yaml",
							Content: "some content",
						},
					},
					"adding workload1",
				),

				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload1/helm-release.yaml",
							Content: "some content",
						},
					},
					"adding a new manifest to workload1"),
			),
			[]detect.WorkloadChange{
				{
					Op: detect.OperationCopy,
					W: detect.Workload{
						Name:      "workload1",
						SourceEnv: "manifests",
					},
				},
			},
			nil,
		},
		"Modifying an workload, adding a new": {
			testutils.RepoWith(t,
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload2/kustomization.yaml",
							Content: "some content",
						},
					},
					"first commit",
				),

				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload1/kustomization.yaml",
							Content: "some content",
						},
						{
							Path:    "flux/manifests/workload2/kustomization.yaml",
							Content: "some NEW content",
						},
					},
					"second commit",
				),
			),
			[]detect.WorkloadChange{
				{
					Op: detect.OperationCopy,
					W: detect.Workload{
						Name:      "workload1",
						SourceEnv: "manifests",
					},
				},
				{
					Op: detect.OperationCopy,
					W: detect.Workload{
						Name:      "workload2",
						SourceEnv: "manifests",
					},
				},
			},

			nil,
		},
		"Renaming a manifest in a workload": {
			testutils.RepoWith(t,
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload2/kustomization.yaml",
							Content: "some content",
						},
					},
					"first commit",
				),
				testutils.RenameContent(
					"flux/manifests/workload2/kustomization.yaml",
					"flux/manifests/workload2/kust.yaml",
					"rename",
				),
			),
			[]detect.WorkloadChange{
				{
					Op: detect.OperationCopy,
					W: detect.Workload{
						Name:      "workload2",
						SourceEnv: "manifests",
					},
				},
			},
			nil,
		},
		"Moving a manifest from one workload to another": {
			testutils.RepoWith(t,
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload2/kustomization.yaml",
							Content: "some content",
						},
						{
							Path:    "flux/manifests/workload2/helm-release.yaml",
							Content: "some content",
						},
						{
							Path:    "flux/manifests/w1/kustomization.yaml",
							Content: "some content",
						},
					},
					"first commit",
				),
				testutils.RenameContent(
					"flux/manifests/workload2/kustomization.yaml",
					"flux/manifests/w1/kust.yaml",
					"rename",
				),
			),
			[]detect.WorkloadChange{
				{
					Op: detect.OperationCopy,
					W: detect.Workload{
						Name:      "w1",
						SourceEnv: "manifests",
					},
				},
				{
					Op: detect.OperationCopy,
					W: detect.Workload{
						Name:      "workload2",
						SourceEnv: "manifests",
					},
				},
			},
			nil,
		},
		"Promotion diff of moving a manifest from one workload to another": {
			testutils.RepoWith(t,
				testutils.AddContent(
					[]testutils.Content{
						//the resources in manifests are to satisfy the environment check which always
						//sees if the source of manifests contains the files or not then the current environment
						{
							Path:    "flux/manifests/workload2/kustomization.yaml",
							Content: "some content",
						},
						{
							Path:    "flux/manifests/workload2/asset.yaml",
							Content: "some content",
						},
						{
							Path:    "flux/manifests/w1/helm-release.yaml",
							Content: "some content",
						},
						{
							Path:    "flux/promoted/development/cloud1/dev2/workload2/kustomization.yaml",
							Content: "some content",
						},
						{
							Path:    "flux/promoted/development/cloud1/dev2/workload2/asset.yaml",
							Content: "some content",
						},
						{
							Path:    "flux/promoted/development/cloud1/dev2/w1/helm-release.yaml",
							Content: "some content",
						},
					},
					"first commit",
				),
				testutils.RenameContent(
					"flux/promoted/development/cloud1/dev2/workload2/asset.yaml",
					"flux/promoted/development/cloud1/dev2/w1/asset.yaml",
					"rename",
				),
			),
			[]detect.WorkloadChange{
				{
					Op: detect.OperationCopy,
					W: detect.Workload{
						Name:      "w1",
						SourceEnv: "development",
					},
				},
				{
					Op: detect.OperationCopy,
					W: detect.Workload{
						Name:      "workload2",
						SourceEnv: "development",
					},
				},
			},
			nil,
		},
		"Deleting a workload, modifying another": {
			testutils.RepoWith(t,
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload2/kustomization.yaml",
							Content: "some content",
						},
					},
					"first commit",
				),

				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload1/kustomization.yaml",
							Content: "some workload1 content",
						},
					},
					"Adding workload1",
				),

				testutils.DeleteContent(
					[]string{"flux/manifests/workload2/kustomization.yaml"},
					"delete workload2",
				),
			),
			[]detect.WorkloadChange{
				{
					Op: detect.OperationCopy,
					W: detect.Workload{
						Name:      "workload1",
						SourceEnv: "manifests",
					},
				},
				{
					Op: detect.OperationRemove,
					W: detect.Workload{
						Name:      "workload2",
						SourceEnv: "manifests",
					},
				},
			},
			nil,
		},
		"Adding a new workload, removing a manifest in another workload, workload still exists": {
			testutils.RepoWith(t,
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload2/helm-release.yaml",
							Content: "some-content relating to a helm release",
						},
						{
							Path:    "flux/manifests/workload2/helm-repository.yaml",
							Content: "some-content relating to a helm repository",
						},
					},
					"initial commit",
				),
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload1/helm-repository.yaml",
							Content: "some-content",
						},
					},
					"Adding workload1 commit",
				),

				testutils.DeleteContent(
					[]string{"flux/manifests/workload2/helm-repository.yaml"},
					"delete manifest in workload2",
				),
			),
			[]detect.WorkloadChange{
				{
					Op: detect.OperationCopy,
					W: detect.Workload{
						SourceEnv: "manifests",
						Name:      "workload1",
					},
				},
				{
					Op: detect.OperationCopy,
					W: detect.Workload{
						SourceEnv: "manifests",
						Name:      "workload2",
					},
				},
			},
			nil,
		},
		"Adding a new workload, removing all manifests in another workload, workload removed": {
			testutils.RepoWith(t,
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload2/helm-release.yaml",
							Content: "some-content relating to a helm release",
						},
						{
							Path:    "flux/manifests/workload2/helm-repository.yaml",
							Content: "some-content relating to a helm repository",
						},
					},
					"initial commit",
				),
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload1/helm-repository.yaml",
							Content: "some-content",
						},
					},
					"Adding workload1 commit",
				),

				testutils.DeleteContent(
					[]string{
						"flux/manifests/workload2/helm-repository.yaml",
						"flux/manifests/workload2/helm-release.yaml",
					},
					"delete manifest in workload2",
				),
			),
			[]detect.WorkloadChange{
				{
					Op: detect.OperationCopy,
					W: detect.Workload{
						SourceEnv: "manifests",
						Name:      "workload1",
					},
				},
				{
					Op: detect.OperationRemove,
					W: detect.Workload{
						SourceEnv: "manifests",
						Name:      "workload2",
					},
				},
			},
			nil,
		},
		"Rename a workload to a new name without changes": {
			testutils.RepoWith(t,
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload2/helm-release.yaml",
							Content: "some-content relating to a helm release",
						},
						{
							Path:    "flux/manifests/workload2/helm-repository.yaml",
							Content: "some-content relating to a helm repository",
						},
					},
					"initial commit",
				),
				testutils.RenameContent(
					"flux/manifests/workload2",
					"flux/manifests/tool-echo",
					"Rename workload",
				),
			),
			[]detect.WorkloadChange{
				{
					Op: detect.OperationCopy,
					W: detect.Workload{
						SourceEnv: "manifests",
						Name:      "tool-echo",
					},
				},
				{
					Op: detect.OperationRemove,
					W: detect.Workload{
						SourceEnv: "manifests",
						Name:      "workload2",
					},
				},
			},
			nil,
		},
		"Rename a workload to a new name with minor changes": {
			testutils.RepoWith(t,
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload2/helm-release.yaml",
							Content: "release name: workload2, some additional content to lower impact on name change",
						},
						{
							Path:    "flux/manifests/workload2/helm-repository.yaml",
							Content: "some-content relating to a helm repository",
						},
					},
					"initial commit",
				),
				testutils.RenameContent(
					"flux/manifests/workload2",
					"flux/manifests/tool-echo",
					"Rename workload",
				),

				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/tool-echo/helm-release.yaml",
							Content: "release name: tool-echo, some additional content to lower impact on name change",
						},
					},
					"Renaming helm release artefact",
				),
			),

			[]detect.WorkloadChange{
				{
					Op: detect.OperationCopy,
					W: detect.Workload{
						SourceEnv: "manifests",
						Name:      "tool-echo",
					},
				},
				{
					Op: detect.OperationRemove,
					W: detect.Workload{
						SourceEnv: "manifests",
						Name:      "workload2",
					},
				},
			},
			nil,
		},
		"Rename a workload to a new name with major changes": {
			testutils.RepoWith(t,
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload2/helm-release.yaml",
							Content: "release name: workload2, some additional content to lower impact on name change",
						},
						{
							Path:    "flux/manifests/workload2/helm-repository.yaml",
							Content: "some-content relating to a helm repository",
						},
					},
					"initial commit",
				),
				testutils.RenameContent(
					"flux/manifests/workload2",
					"flux/manifests/tool-echo",
					"Rename workload",
				),

				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/tool-echo/helm-release.yaml",
							Content: "release name: tool-echo, completely new content, not the same as before, at all, adding a few more notes to make sure it's beyond 60% change",
						},
					},
					"Renaming helm release artefact",
				),
			),

			[]detect.WorkloadChange{
				{
					Op: detect.OperationCopy,
					W: detect.Workload{
						SourceEnv: "manifests",
						Name:      "tool-echo",
					},
				},
				{
					Op: detect.OperationRemove,
					W: detect.Workload{
						SourceEnv: "manifests",
						Name:      "workload2",
					},
				},
			},
			nil,
		},
		"Workload rename in promoted directory": {
			testutils.RepoWith(t,
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/promoted/development/cloud1/dev2/workload2/helm-release.yaml",
							Content: "release name: workload2, some additional content to lower impact on name change",
						},
						{
							Path:    "flux/promoted/development/cloud1/dev2/workload2/helm-repository.yaml",
							Content: "some-content relating to a helm repository",
						},
					},
					"initial commit",
				),
				testutils.RenameContent(
					"flux/promoted/development/cloud1/dev2/workload2",
					"flux/promoted/development/cloud1/dev2/tool-echo",
					"Rename workload",
				),
			),

			[]detect.WorkloadChange{
				{
					Op: detect.OperationCopy,
					W: detect.Workload{
						SourceEnv: "development",
						Name:      "tool-echo",
					},
				},
				{
					Op: detect.OperationRemove,
					W: detect.Workload{
						SourceEnv: "development",
						Name:      "workload2",
					},
				},
			},
			nil,
		},
		"Workload and non-workload files are updated": {
			testutils.RepoWith(t,
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload2/kustomization.yaml",
							Content: "some content",
						},
					},
					"first commit",
				),

				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload1/kustomization.yaml",
							Content: "some content",
						},
					},
					"adding workload1",
				),

				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "/README.md",
							Content: "Documentation update",
						},
					},
					"Updating README.md",
				),
			),
			[]detect.WorkloadChange{
				{
					Op: detect.OperationCopy,
					W: detect.Workload{
						Name:      "workload1",
						SourceEnv: "manifests",
					},
				},
			},
			nil,
		},
		"Only non-workload files are updated": {
			testutils.RepoWith(t,
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload2/kustomization.yaml",
							Content: "some content",
						},
					},
					"first commit",
				),

				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "/README.md",
							Content: "Documentation update",
						},
					},
					"Updating README.md",
				),
			),
			[]detect.WorkloadChange{},
			detect.ErrNoChange,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			l := logrus.NewEntry(logrus.New())
			d, err := detect.NewDetect(tt.TestRepo.Repo, tt.TestRepo.CommitRange(), dummyWorkloadRegistry{}, l)
			require.NoError(t, err)

			got, err := d.WorkloadChange()
			if err == nil {
				require.NoError(t, err, "diff reported: %v", err)
				require.Equal(t, tt.expect, got)
			} else {
				require.ErrorIs(t, err, tt.err)
			}
		})
	}
}

func TestGetSourceCommits(t *testing.T) {
	tests := map[string]struct {
		TestRepo *testutils.TestRepo
		expect   []*github.Commit
	}{
		"Source commits exist": {
			testutils.RepoWith(t,
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload2/kustomization.yaml",
							Content: "initial content",
						},
					},
					"initial commit\n"+
						"\n"+
						"Source-commit: b4a645041ce260ee8da79c8e3850841452cece2e A:user0-form3 C:user0-form3",
				),
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload2/kustomization.yaml",
							Content: "content 1",
						},
					},
					"commit 1\n"+
						"\n"+
						"Source-commit: 30691ee94e97dae5404e48276bd5905ec27dee26 A:user1-form3 C:user2-form3\n"+
						"Source-commit: d535711c1c1793fac5b75d83e68c92128a1b9da0 A:user1-form3 C:user3-form3",
				),
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload2/kustomization.yaml",
							Content: "content 2",
						},
					},
					"commit 2\n"+
						"\n"+
						"Source-commit: 58663c35b4b18e7f99e268f5e246d1de39627145 A:user3-form3 C:user2-form3",
				),
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload2/kustomization.yaml",
							Content: "content 3",
						},
					},
					// Pressing `Squash and merge` on a PR can end up with something like this
					"commit 3\r\n"+
						"\r\n"+
						"Source-commit: d535711c1c1793fac5b75d83e68c92128a1b9da0 A:user4-form3 C:web-flow\r\n"+
						"Co-authored-by: user4-form3 <user1-form4@form3.tech>",
				),
			),
			[]*github.Commit{
				{
					Hash:           "d535711c1c1793fac5b75d83e68c92128a1b9da0",
					AuthorLogin:    "user4-form3",
					CommitterLogin: "web-flow",
				},
				{
					Hash:           "58663c35b4b18e7f99e268f5e246d1de39627145",
					AuthorLogin:    "user3-form3",
					CommitterLogin: "user2-form3",
				},
				{
					Hash:           "30691ee94e97dae5404e48276bd5905ec27dee26",
					AuthorLogin:    "user1-form3",
					CommitterLogin: "user2-form3",
				},
				{
					Hash:           "d535711c1c1793fac5b75d83e68c92128a1b9da0",
					AuthorLogin:    "user1-form3",
					CommitterLogin: "user3-form3",
				},
			},
		},
		"Source commits missing": {
			testutils.RepoWith(t,
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload2/kustomization.yaml",
							Content: "initial content",
						},
					},
					"initial commit",
				),
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload2/kustomization.yaml",
							Content: "content 1",
						},
					},
					"commit 1",
				),
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/manifests/workload2/kustomization.yaml",
							Content: "content 2",
						},
					},
					"commit 2",
				),
			),
			[]*github.Commit{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			l := logrus.NewEntry(logrus.New())
			d, err := detect.NewDetect(tt.TestRepo.Repo, tt.TestRepo.CommitRange(), dummyWorkloadRegistry{}, l)
			require.NoError(t, err)
			got, err := d.GetSourceCommits()

			require.NoError(t, err, "getSourceCommits reported: %v", err)
			require.Equal(t, tt.expect, got)
		})
	}
}

type dummyWorkloadRegistry struct{}

func (d dummyWorkloadRegistry) Get(workloadID string) (clusterconf.Workload, error) {
	return clusterconf.Workload{}, nil
}

func (d dummyWorkloadRegistry) GetAll() ([]clusterconf.Workload, error) {
	return nil, nil
}

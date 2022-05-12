package clusterconf_test

import (
	"fmt"
	"testing"

	"github.com/form3tech/k8s-promoter/internal/clusterconf"
	"github.com/form3tech/k8s-promoter/internal/environment"
	"github.com/form3tech/k8s-promoter/internal/testutils"
	"github.com/go-git/go-git/v5"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestCluster_New(t *testing.T) {
	tests := map[string]struct {
		gitRepo     *git.Repository
		expected    *clusterconf.ClusterInspecter
		expectedErr error
	}{
		"when repo is nil": {
			gitRepo:     nil,
			expected:    nil,
			expectedErr: clusterconf.ErrRepoNotInitialised,
		},
		"when valid repo": {
			gitRepo: &git.Repository{},
			expected: &clusterconf.ClusterInspecter{
				Repo: &git.Repository{},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			l := logrus.NewEntry(logrus.New())
			got, err := clusterconf.NewClusterInspecter(tt.gitRepo, l)
			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
				require.Empty(t, got)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected.Repo, got.Repo)
		})
	}
}

func TestCluster_Detect(t *testing.T) {
	tests := map[string]struct {
		repo                *testutils.TestRepo
		allClusters         []clusterconf.Cluster
		newClusters         []clusterconf.Cluster
		previousEnvClusters []clusterconf.Cluster
		env                 environment.Env
	}{
		"no drift: empty upstream clusters + empty repo": {
			repo:                testutils.RepoWith(t),
			allClusters:         nil,
			newClusters:         nil,
			previousEnvClusters: nil,
			env:                 environment.Development,
		},
		"no drift: upstream clusters present on repo": {
			repo: testutils.RepoWith(t,
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/promoted/development/cluster-1/kustomization.yaml",
							Content: "some content",
						},
						{
							Path:    "flux/promoted/development/cluster-2/kustomization.yaml",
							Content: "some content",
						},
					},
					"first commit",
				),
			),
			allClusters: []clusterconf.Cluster{
				cluster("cluster-1", environment.Development),
				cluster("cluster-2", environment.Development),
			},
			newClusters:         []clusterconf.Cluster{},
			previousEnvClusters: []clusterconf.Cluster{},
			env:                 environment.Development,
		},
		"test env no drift: upstream clusters present on repo": {
			repo: testutils.RepoWith(t,
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/promoted/development/cluster-1/kustomization.yaml",
							Content: "some content",
						},
						{
							Path:    "flux/promoted/development/cluster-2/kustomization.yaml",
							Content: "some content",
						},
						{
							Path:    "flux/promoted/test/cluster-test-1/kustomization.yaml",
							Content: "some content",
						},
					},
					"first commit",
				),
			),
			allClusters: []clusterconf.Cluster{
				cluster("cluster-1", environment.Development),
				cluster("cluster-2", environment.Development),
				cluster("cluster-test-1", environment.Test),
			},
			newClusters: []clusterconf.Cluster{},
			previousEnvClusters: []clusterconf.Cluster{
				cluster("cluster-1", environment.Development),
				cluster("cluster-2", environment.Development),
			},
			env: environment.Test,
		},
		"drift: not empty upstream clusters + empty repo": {
			repo: testutils.RepoWith(t),
			allClusters: []clusterconf.Cluster{
				cluster("cluster-1", environment.Development),
				cluster("cluster-2", environment.Development),
			},
			newClusters: []clusterconf.Cluster{
				cluster("cluster-1", environment.Development),
				cluster("cluster-2", environment.Development),
			},
			previousEnvClusters: []clusterconf.Cluster{},
			env:                 environment.Development,
		},
		"drift: not empty upstream clusters + not empty repo": {
			repo: testutils.RepoWith(t,
				testutils.AddContent(
					[]testutils.Content{
						{
							Path:    "flux/promoted/development/cluster-1/kustomization.yaml",
							Content: "some content",
						},
						{
							Path:    "flux/promoted/development/cluster-2/kustomization.yaml",
							Content: "some content",
						},
					},
					"first commit",
				),
			),
			allClusters: []clusterconf.Cluster{
				cluster("cluster-1", environment.Development),
				cluster("cluster-2", environment.Development),
				cluster("cluster-3", environment.Development),
			},
			newClusters: []clusterconf.Cluster{
				cluster("cluster-3", environment.Development),
			},
			previousEnvClusters: []clusterconf.Cluster{},
			env:                 environment.Development,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := &clusterconf.ClusterInspecter{
				Repo: tt.repo.Repo,
			}
			got, err := c.Detect(tt.allClusters, tt.env)
			require.NoError(t, err)
			require.ElementsMatch(t, tt.newClusters, got.New)
			require.ElementsMatch(t, tt.previousEnvClusters, got.PreviousEnv)
		})
	}
}

func cluster(name string, env environment.Env) clusterconf.Cluster {
	return clusterconf.Cluster{
		Spec: clusterconf.ClusterSpec{
			ManifestFolder: fmt.Sprintf("flux/promoted/%s/%s", string(env), name),
		},
		Metadata: clusterconf.ClusterMetadata{
			Name:   name,
			Labels: clusterconf.Labels{"environment": string(env)},
		},
	}
}

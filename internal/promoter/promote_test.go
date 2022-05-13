package promoter_test

import (
	"testing"

	"github.com/form3tech/k8s-promoter/internal/environment"
	"github.com/form3tech/k8s-promoter/internal/promoter"
	"github.com/sirupsen/logrus"
)

func Test_InitialPromotionOfManifestsToDevelopment(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		a_clusters_configuration_file().
		commit_range_start().
		new_source_manifests_for_the_workload("foo").
		commit_range_end()

	when.
		promote().
		with_env(environment.Development).
		is_called()

	then.
		promote_succeeds().
		the_remote_repository_is_updated_with_new_branch().
		the_number_of_raised_PRs_equals(1)

	then.
		a_PR_for("foo", environment.Development, "dev2-cloud1", "dev3-cloud1", "dev4-cloud2").
		has_labels("k8s-promoter/automated-promotion").
		has_no_assignees().
		has_branch().with_one_commit().
		that_contains_updated_foo_manifests_for_clusters(
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1",
			"/promoted/development/dev4/cloud2").
		that_has_kustomization_for_workloads("/promoted/development/dev2/cloud1", "foo").
		that_has_kustomization_for_workloads("/promoted/development/dev3/cloud1", "foo").
		that_has_kustomization_for_workloads("/promoted/development/dev4/cloud2", "foo").
		that_contains_foo_changes_only_for_directories(
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1",
			"/promoted/development/dev4/cloud2")
}

func Test_PromotionOfManifestsToDevelopment(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		a_clusters_configuration_file().
		old_source_manifests_for_the_workload("foo").
		old_dev_manifests_for_the_workload_foo().
		commit_range_start().
		new_source_manifests_for_the_workload("foo").
		commit_range_end()

	when.
		promote().
		with_env(environment.Development).
		is_called()

	then.
		promote_succeeds().
		the_remote_repository_is_updated_with_new_branch().
		the_number_of_raised_PRs_equals(1)

	then.
		a_PR_for("foo", environment.Development, "dev2-cloud1", "dev3-cloud1", "dev4-cloud2").
		has_labels("k8s-promoter/automated-promotion").
		has_assignees("test-user-2", "test-user-3").
		has_branch().with_one_commit().with_source_commit().
		that_contains_updated_foo_manifests_for_clusters(
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1",
			"/promoted/development/dev4/cloud2").
		that_has_kustomization_for_workloads("/promoted/development/dev2/cloud1", "foo").
		that_has_kustomization_for_workloads("/promoted/development/dev3/cloud1", "foo").
		that_has_kustomization_for_workloads("/promoted/development/dev4/cloud2", "foo").
		that_contains_foo_changes_only_for_directories(
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1",
			"/promoted/development/dev4/cloud2")
}

func Test_CommitRangeIncludesNonWorkloadRelatedFiles(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		a_clusters_configuration_file().
		commit_range_start().
		new_source_manifests_for_the_workload("foo").
		non_workload_update("/README.md", "new readme content").
		commit_range_end()

	when.
		promote().
		with_env(environment.Development).
		is_called()

	then.
		promote_succeeds().
		the_remote_repository_is_updated_with_new_branch().
		the_number_of_raised_PRs_equals(1)

	then.
		a_PR_for("foo", environment.Development, "dev2-cloud1", "dev3-cloud1", "dev4-cloud2").
		has_labels("k8s-promoter/automated-promotion").
		has_branch().with_one_commit().
		that_contains_updated_foo_manifests_for_clusters(
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1",
			"/promoted/development/dev4/cloud2").
		that_has_kustomization_for_workloads("/promoted/development/dev2/cloud1", "foo").
		that_has_kustomization_for_workloads("/promoted/development/dev3/cloud1", "foo").
		that_has_kustomization_for_workloads("/promoted/development/dev4/cloud2", "foo").
		that_contains_foo_changes_only_for_directories(
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1",
			"/promoted/development/dev4/cloud2")
}

func Test_CommitRangeContainsOnlyNonWorkloadRelatedFiles(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		a_clusters_configuration_file().
		commit_range_start().
		a_file_with_content(".travis.yaml", "some-config").
		non_workload_update("README.md", "new readme content").
		commit_range_end()

	when.
		promote().
		with_env(environment.Development).
		is_called()

	then.
		promote_succeeds().
		a_message_is_logged(promoter.NoChangesMsg, logrus.InfoLevel)

	then.
		the_remote_repository_is_not_updated_with_new_branch().
		the_number_of_raised_PRs_equals(0)
}

func Test_CommitRangeIncludesMultipleSourceEnvironments(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		a_clusters_configuration_file().
		old_source_manifests_for_the_workload("foo").
		commit_range_start().
		old_dev_manifests_for_the_workload_foo().
		old_test_manifests_for_the_workload_foo().
		new_source_manifests_for_the_workload("bar").
		commit_range_end()

	when.
		promote().
		with_env(environment.Development).
		is_called()

	then.
		promote_succeeds().
		the_remote_repository_is_updated_with_new_branch().
		the_number_of_raised_PRs_equals(1)

	then.
		a_PR_for("bar", environment.Development, "dev2-cloud1", "dev3-cloud1", "dev4-cloud2").
		has_labels("k8s-promoter/automated-promotion").
		has_branch().with_one_commit().
		// Only one source commit should be seen here as there is only one source manifest change. However, the current
		// implementation isn't smart enough to handle that and ends up thinking that all three manifest changes are
		// source commits. As this isn't a normal scenario and the implementation is non-trivial to fix, we don't:
		// with_source_commit().
		that_contains_updated_bar_manifests_for_clusters(
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1",
			"/promoted/development/dev4/cloud2").
		that_has_kustomization_for_workloads("/promoted/development/dev2/cloud1", "bar", "foo").
		that_has_kustomization_for_workloads("/promoted/development/dev3/cloud1", "bar", "foo").
		that_has_kustomization_for_workloads("/promoted/development/dev4/cloud2", "bar", "foo").
		that_contains_bar_changes_only_for_directories(
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1",
			"/promoted/development/dev4/cloud2")
}

func Test_CommitRangeIncludesOnlyOtherSourceEnvironments(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		a_clusters_configuration_file().
		commit_range_start().
		new_source_manifests_for_the_workload("foo").
		new_test_manifests_for_the_workload_foo().
		commit_range_end()

	when.
		promote().
		with_env(environment.Test).
		is_called()

	then.
		promote_succeeds().
		a_message_is_logged(promoter.NoChangesMsg, logrus.InfoLevel)
}

func Test_InitialPromotionToTestOfCorrectlyPromotedDevelopmentWorkloads(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		a_clusters_configuration_file().
		old_source_manifests_for_the_workload("foo").
		new_source_manifests_for_the_workload("foo").
		commit_range_start().
		new_dev_manifests_for_the_workload_foo().
		commit_range_end()

	when.
		promote().
		with_env(environment.Test).
		is_called()

	then.
		promote_succeeds().
		the_remote_repository_is_updated_with_3_new_branches().
		the_number_of_raised_PRs_equals(3)

	// Given this is first promotion to test, we don't have the directly structure
	// for tests clusters on disk. Because of that, promoter treats it as promotion
	// to new cluster. This causes the resulting pull request to not have any assigness.
	then.
		a_PR_for("foo", environment.Test, "test1-cloud1").
		has_labels("k8s-promoter/automated-promotion").
		has_no_assignees().
		has_branch().with_one_commit().
		that_contains_updated_foo_manifests_for_cluster("/promoted/test/test1/cloud1").
		that_contains_changes_only_for_directory("/promoted/test/test1/cloud1").
		that_has_kustomization_for_workloads("/promoted/test/test1/cloud1", "foo")

	then.
		a_PR_for("foo", environment.Test, "test2-cloud1").
		has_labels("k8s-promoter/automated-promotion").
		has_no_assignees().
		has_branch().with_one_commit().
		that_contains_updated_foo_manifests_for_cluster("/promoted/test/test2/cloud1").
		that_contains_changes_only_for_directory("/promoted/test/test2/cloud1").
		that_has_kustomization_for_workloads("/promoted/test/test2/cloud1", "foo")

	then.
		a_PR_for("foo", environment.Test, "test3-cloud2").
		has_labels("k8s-promoter/automated-promotion").
		has_no_assignees().
		has_branch().with_one_commit().
		that_contains_updated_foo_manifests_for_cluster("/promoted/test/test3/cloud2").
		that_contains_changes_only_for_directory("/promoted/test/test3/cloud2").
		that_has_kustomization_for_workloads("/promoted/test/test3/cloud2", "foo")
}

func Test_PromoteWorkloadUpdatesWhenWorkloadsExistInTest(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		a_clusters_configuration_file().
		old_dev_manifests_for_the_workload_foo().
		old_test_manifests_for_the_workload_foo().
		new_source_manifests_for_the_workload("foo").
		commit_range_start().
		new_dev_manifests_for_the_workload_foo().
		commit_range_end()

	when.
		promote().
		with_env(environment.Test).
		is_called()

	then.
		promote_succeeds().
		the_remote_repository_is_updated_with_3_new_branches().
		the_number_of_raised_PRs_equals(3)

	// Contrary to the previous tests, here we already have a correct directory structure on disk.
	// Therefore, the resulting pull request has assignees.
	then.
		a_PR_for("foo", environment.Test, "test1-cloud1").
		has_labels("k8s-promoter/automated-promotion").
		has_assignees("test-user-2", "test-user-3").
		has_branch().with_one_commit().with_source_commit().
		that_contains_updated_foo_manifests_for_cluster("/promoted/test/test1/cloud1").
		that_contains_changes_only_for_directory("/promoted/test/test1/cloud1").
		that_has_kustomization_for_workloads("/promoted/test/test1/cloud1", "foo")

	then.
		a_PR_for("foo", environment.Test, "test2-cloud1").
		has_labels("k8s-promoter/automated-promotion").
		has_assignees("test-user-2", "test-user-3").
		has_branch().with_one_commit().with_source_commit().
		that_contains_updated_foo_manifests_for_cluster("/promoted/test/test2/cloud1").
		that_contains_changes_only_for_directory("/promoted/test/test2/cloud1").
		that_has_kustomization_for_workloads("/promoted/test/test2/cloud1", "foo")

	then.
		a_PR_for("foo", environment.Test, "test3-cloud2").
		has_labels("k8s-promoter/automated-promotion").
		has_assignees("test-user-2", "test-user-3").
		has_branch().with_one_commit().with_source_commit().
		that_contains_updated_foo_manifests_for_cluster("/promoted/test/test3/cloud2").
		that_contains_changes_only_for_directory("/promoted/test/test3/cloud2").
		that_has_kustomization_for_workloads("/promoted/test/test3/cloud2", "foo")
}

func Test_InitalPromotionOfWorkloadsToProduction(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		a_clusters_configuration_file().
		new_source_manifests_for_the_workload("foo").
		new_dev_manifests_for_the_workload_foo().
		commit_range_start().
		new_test_manifests_for_the_workload_foo().
		commit_range_end()

	when.
		promote().
		with_env(environment.Production).
		is_called()
	then.
		promote_succeeds().
		the_remote_repository_is_updated_with_3_new_branches().
		the_number_of_raised_PRs_equals(3)

	then.
		a_PR_for("foo", environment.Production, "prod1-cloud1").
		has_labels("k8s-promoter/automated-promotion").
		has_no_assignees().
		has_branch().with_one_commit().
		that_contains_updated_foo_manifests_for_cluster("/promoted/production/prod1/cloud1").
		that_contains_changes_only_for_directory("/promoted/production/prod1/cloud1").
		that_has_kustomization_for_workloads("/promoted/production/prod1/cloud1", "foo")

	then.
		a_PR_for("foo", environment.Production, "prod2-cloud1").
		has_labels("k8s-promoter/automated-promotion").
		has_no_assignees().
		has_branch().with_one_commit().
		that_contains_updated_foo_manifests_for_cluster("/promoted/production/prod2/cloud1").
		that_contains_changes_only_for_directory("/promoted/production/prod2/cloud1").
		that_has_kustomization_for_workloads("/promoted/production/prod2/cloud1", "foo")

	then.
		a_PR_for("foo", environment.Production, "prod3-cloud1").
		has_labels("k8s-promoter/automated-promotion").
		has_no_assignees().
		has_branch().with_one_commit().
		that_contains_updated_foo_manifests_for_cluster("/promoted/production/prod3/cloud1").
		that_contains_changes_only_for_directory("/promoted/production/prod3/cloud1").
		that_has_kustomization_for_workloads("/promoted/production/prod3/cloud1", "foo")
}

func Test_PromotionWhenWorkloadAddedAndRemoved(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		a_clusters_configuration_file().
		commit_range_start().
		new_source_manifests_for_the_workload("bar").
		new_source_manifests_for_the_workload("foo").
		deleted_source_manifests_for_the_workload("bar").
		commit_range_end()

	when.
		promote().
		with_env(environment.Development).
		is_called()

	then.
		promote_succeeds().
		the_remote_repository_is_updated_with_new_branch().
		the_number_of_raised_PRs_equals(1)

	then.
		a_PR_for("foo", environment.Development, "dev2-cloud1", "dev3-cloud1", "dev4-cloud2").
		has_labels("k8s-promoter/automated-promotion").
		has_no_assignees().
		has_branch().with_one_commit().
		that_contains_updated_foo_manifests_for_clusters(
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1",
			"/promoted/development/dev4/cloud2").
		that_has_kustomization_for_workloads("/promoted/development/dev2/cloud1", "foo").
		that_has_kustomization_for_workloads("/promoted/development/dev3/cloud1", "foo").
		that_has_kustomization_for_workloads("/promoted/development/dev4/cloud2", "foo").
		that_contains_foo_changes_only_for_directories(
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1",
			"/promoted/development/dev4/cloud2")
}

func Test_PromotionOfRemovalOfExistingWorkload(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		a_clusters_configuration_file().
		old_source_manifests_for_the_workload("foo").
		old_dev_manifests_for_the_workload_foo().
		commit_range_start().
		deleted_source_manifests_for_the_workload("foo").
		commit_range_end()

	when.
		promote().
		with_env(environment.Development).
		is_called()

	then.
		promote_succeeds().
		the_remote_repository_is_updated_with_new_branch().
		the_number_of_raised_PRs_equals(1)

	then.
		a_PR_for("foo", environment.Development, "dev2-cloud1", "dev3-cloud1", "dev4-cloud2").
		has_labels("k8s-promoter/automated-promotion").
		has_assignees("test-user-3", "test-user-4").
		has_branch().with_one_commit().with_source_commit().
		that_deletes_manifests(
			"foo",
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1",
			"/promoted/development/dev4/cloud2",
		).
		that_deletes_kustomization_for_workload(
			"foo",
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1",
			"/promoted/development/dev4/cloud2",
		)
}

func Test_PromotionOfManifestRenamesForWorkload(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		a_clusters_configuration_file().
		old_source_manifests_for_the_workload("foo").
		commit_range_start().
		source_manifest_renamed_in_the_workload("foo").
		commit_range_end()

	when.
		promote().
		with_env(environment.Development).
		is_called()

	then.
		promote_succeeds().
		the_remote_repository_is_updated_with_new_branch().
		the_number_of_raised_PRs_equals(1)

	then.
		a_PR_for("foo", environment.Development, "dev2-cloud1", "dev3-cloud1", "dev4-cloud2").
		has_labels("k8s-promoter/automated-promotion").
		has_no_assignees().
		has_branch().with_one_commit().
		that_contains_foo_changes_only_for_directories(
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1",
			"/promoted/development/dev4/cloud2").
		that_contains_renamed_manifest_only_in_the_workload(
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1",
			"/promoted/development/dev4/cloud2")
}

func Test_PromotionOfManifestRenamesForAlreadyPromotedWorkload(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		a_clusters_configuration_file().
		old_source_manifests_for_the_workload("foo").
		old_dev_manifests_for_the_workload_foo().
		commit_range_start().
		manifest_for_workload_foo_is_renamed_to_bar().
		commit_range_end()

	when.
		promote().
		with_env(environment.Development).
		is_called()

	then.
		promote_succeeds().
		the_remote_repository_is_updated_with_new_branch().
		the_number_of_raised_PRs_equals(1)

	then.
		a_PR_for("bar, foo", environment.Development, "dev2-cloud1", "dev3-cloud1", "dev4-cloud2").
		has_assignees("test-user-1", "test-user-4").
		has_labels("k8s-promoter/automated-promotion").
		has_branch().with_one_commit().with_source_commit().
		that_contains_bar_changes_only_for_directories(
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1",
			"/promoted/development/dev4/cloud2").
		that_contains_renamed_workload(
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1",
			"/promoted/development/dev4/cloud2")
}

func Test_PromotionGroupsDevelopmentsClusters(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		old_dev_manifests_for_the_workload_foo().
		a_clusters_configuration_file().
		commit_range_start().
		new_source_manifests_for_the_workload("foo").
		commit_range_end()

	when.
		promote().
		with_env(environment.Development).
		is_called()

	then.
		promote_succeeds().
		the_remote_repository_is_updated_with_new_branch().
		the_number_of_raised_PRs_equals(1)

	then.
		a_PR_for("foo", environment.Development, "dev2-cloud1", "dev3-cloud1", "dev4-cloud2").
		has_labels("k8s-promoter/automated-promotion").
		has_assignees("test-user-2", "test-user-3").
		has_branch().with_one_commit().with_source_commit().
		that_contains_updated_foo_manifests_for_clusters(
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1",
			"/promoted/development/dev4/cloud2").
		that_contains_foo_changes_only_for_directories(
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1",
			"/promoted/development/dev4/cloud2").
		that_has_kustomization_for_workloads("/promoted/development/dev2/cloud1", "foo").
		that_has_kustomization_for_workloads("/promoted/development/dev3/cloud1", "foo").
		that_has_kustomization_for_workloads("/promoted/development/dev4/cloud2", "foo")
}

func Test_PromotionExcludeUserIssueAssignment(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		old_dev_manifests_for_the_workload_foo().
		a_clusters_configuration_file().
		commit_range_start().
		new_source_manifests_for_the_workload("foo").
		commit_range_end()

	when.
		promote().
		with_env(environment.Development).
		with_no_issue_users("test-user-2").
		is_called()

	then.
		promote_succeeds().
		the_remote_repository_is_updated_with_new_branch().
		the_number_of_raised_PRs_equals(1)

	then.
		a_PR_for("foo", environment.Development, "dev2-cloud1", "dev3-cloud1", "dev4-cloud2").
		has_labels("k8s-promoter/automated-promotion").
		has_assignees("test-user-3").
		has_branch().with_one_commit().with_source_commit().
		that_contains_updated_foo_manifests_for_clusters(
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1",
			"/promoted/development/dev4/cloud2").
		that_contains_foo_changes_only_for_directories(
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1",
			"/promoted/development/dev4/cloud2").
		that_has_kustomization_for_workloads("/promoted/development/dev2/cloud1", "foo").
		that_has_kustomization_for_workloads("/promoted/development/dev3/cloud1", "foo").
		that_has_kustomization_for_workloads("/promoted/development/dev4/cloud2", "foo")
}

func Test_PromotionOfWorkloadsWithWorkloadExclusion(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		a_clusters_configuration_file().
		an_cloud1_only_config_file_for_foo().
		commit_range_start().
		new_source_manifests_for_the_workload("foo").
		commit_range_end()

	when.
		promote().
		with_env(environment.Development).
		is_called()

	then.
		promote_succeeds().
		the_remote_repository_is_updated_with_new_branch().
		the_number_of_raised_PRs_equals(1)

	// "foo" is cloud1 only workload, so no pull requests for clusters in cloud2
	then.
		a_PR_for("foo", environment.Development, "dev2-cloud1", "dev3-cloud1").
		has_labels("k8s-promoter/automated-promotion").
		has_no_assignees().
		has_branch().with_one_commit().
		that_contains_updated_foo_manifests_for_clusters(
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1").
		that_contains_foo_changes_only_for_directories(
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1").
		that_has_kustomization_for_workloads("/promoted/development/dev2/cloud1", "foo").
		that_has_kustomization_for_workloads("/promoted/development/dev3/cloud1", "foo")
}

func Test_PromotionOfWorkloadsWithWorkloadExclusionToTestEnv(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		a_clusters_configuration_file().
		an_cloud1_only_config_file_for_foo().
		new_source_manifests_for_the_workload("foo").
		commit_range_start().
		new_dev_manifests_for_the_workload_foo().
		commit_range_end()

	when.
		promote().
		with_env(environment.Test).
		is_called()

	then.
		promote_succeeds().
		the_remote_repository_is_updated_with_2_new_branches().
		the_number_of_raised_PRs_equals(2)

	// "foo" is cloud1 only workload, so no pull requests for clusters in cloud2
	then.
		a_PR_for("foo", environment.Test, "test1-cloud1").
		has_labels("k8s-promoter/automated-promotion").
		has_no_assignees().
		has_branch().with_one_commit().
		that_contains_updated_foo_manifests_for_cluster("/promoted/test/test1/cloud1").
		that_contains_changes_only_for_directory("/promoted/test/test1/cloud1").
		that_has_kustomization_for_workloads("/promoted/test/test1/cloud1", "foo")

	then.
		a_PR_for("foo", environment.Test, "test2-cloud1").
		has_labels("k8s-promoter/automated-promotion").
		has_no_assignees().
		has_branch().with_one_commit().
		that_contains_updated_foo_manifests_for_cluster("/promoted/test/test2/cloud1").
		that_contains_changes_only_for_directory("/promoted/test/test2/cloud1").
		that_has_kustomization_for_workloads("/promoted/test/test2/cloud1", "foo")
}

func Test_PromotionToTestWhenDevelopmentInInconsistentState(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		a_clusters_configuration_file().
		new_source_manifests_for_the_workload("foo").
		new_dev_manifests_for_the_workload_foo().
		commit_range_start().
		a_file_with_content(path("/promoted/development/dev2/cloud1/foo/file"), "inconsistent").
		commit_range_end()

	when.
		promote().
		with_env(environment.Test).
		is_called()

	then.
		promote_succeeds().
		a_message_is_logged(promoter.NotInSyncMsg, logrus.InfoLevel)
}

func Test_PromotionToProductionWhenTestInInconsistentState(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		a_clusters_configuration_file_with_new_prod_clusters().
		old_source_manifests_for_the_workload("foo").
		old_dev_manifests_for_the_workload_foo().
		old_test_manifests_for_the_workload_foo().
		old_prod_manifests_for_the_workload_foo().
		a_file_with_content(path("/promoted/test/test1/cloud1/foo/file"), "inconsistent").
		empty_commit_range()

	when.
		promote().
		with_env(environment.Production).
		is_called()

	then.
		promote_succeeds().
		a_message_is_logged(promoter.NotInSyncMsg, logrus.InfoLevel)
}

func Test_PromotionToNonExistingEnvironment(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		a_clusters_file_with_only_dev_clusters().
		commit_range_start().
		new_dev_manifests_for_the_workload_foo().
		commit_range_end()

	when.
		promote().
		with_env(environment.Test).
		is_called()

	then.
		promote_succeeds().
		a_message_is_logged(promoter.NoClustersMsg, logrus.InfoLevel)
}

func Test_PromotionOfNewDevClusterWithNoManualManifestsChanges(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		old_source_manifests_for_the_workload("bar").
		old_dev_manifests_for_the_workload_bar().
		a_clusters_configuration_file_with_new_dev_cluster().
		empty_commit_range()

	when.
		promote().
		with_env(environment.Development).
		is_called()

	then.
		promote_succeeds().
		the_remote_repository_is_updated_with_new_branch().
		the_number_of_raised_PRs_equals(1)

	then.
		a_PR_for("bar", environment.Development, "dev1-cloud1").
		has_labels("k8s-promoter/automated-promotion").
		has_branch().with_one_commit().
		that_contains_bar_manifests_for_clusters(
			"/promoted/development/dev1/cloud1").
		that_has_kustomization_for_workloads("/promoted/development/dev1/cloud1", "bar").
		that_contains_bar_changes_only_for_directories(
			"/promoted/development/dev1/cloud1",
			"/config/development/dev1/cloud1/",
		)
}

func Test_PromotionOfNewDevClusterWithManualManifestsChanges(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		old_source_manifests_for_the_workload("bar").
		old_dev_manifests_for_the_workload_bar().
		a_clusters_configuration_file_with_new_dev_cluster().
		commit_range_start().
		new_source_manifests_for_the_workload("bar").
		commit_range_end()

	when.
		promote().
		with_env(environment.Development).
		is_called()

	then.
		promote_succeeds().
		the_remote_repository_is_updated_with_new_branches(2).
		the_number_of_raised_PRs_equals(2)

	then.
		a_PR_for("bar", environment.Development, "dev1-cloud1").
		has_labels("k8s-promoter/automated-promotion").
		has_branch().with_one_commit().
		that_contains_updated_bar_manifests_for_clusters(
			"/promoted/development/dev1/cloud1").
		that_has_kustomization_for_workloads("/promoted/development/dev1/cloud1", "bar").
		that_contains_bar_changes_only_for_directories(
			"/promoted/development/dev1/cloud1",
			"/config/development/dev1/cloud1/",
		)

	then.
		a_PR_for("bar", environment.Development, "dev2-cloud1", "dev3-cloud1", "dev4-cloud2").
		has_labels("k8s-promoter/automated-promotion").
		has_branch().with_one_commit().
		that_contains_updated_bar_manifests_for_clusters(
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1",
			"/promoted/development/dev4/cloud2").
		that_has_kustomization_for_workloads("/promoted/development/dev2/cloud1", "bar").
		that_has_kustomization_for_workloads("/promoted/development/dev3/cloud1", "bar").
		that_has_kustomization_for_workloads("/promoted/development/dev4/cloud2", "bar").
		that_contains_bar_changes_only_for_directories(
			"/promoted/development/dev2/cloud1",
			"/promoted/development/dev3/cloud1",
			"/promoted/development/dev4/cloud2",
		)
}

func Test_PromotionOfNewTestClustersNoManualManifestsChanges(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		old_dev_manifests_for_the_workload_foo().
		old_test_manifests_for_the_workload_foo().
		new_source_manifests_for_the_workload("foo").
		new_dev_manifests_for_the_workload_foo().
		commit_range_start().
		a_clusters_configuration_file_with_new_test_clusters().
		commit_range_end()

	when.
		promote().
		with_env(environment.Test).
		is_called()

	then.
		promote_succeeds().
		the_remote_repository_is_updated_with_2_new_branches().
		the_number_of_raised_PRs_equals(2)

	then.
		a_PR_for("foo", environment.Test, "new-test-cluster-1-cloud1").
		has_labels("k8s-promoter/automated-promotion").
		has_no_assignees().
		has_branch().with_one_commit().
		that_has_kustomization_for_workloads("/promoted/test/new-test-cluster-1/cloud1", "foo").
		that_contains_updated_foo_manifests_for_cluster("/promoted/test/new-test-cluster-1/cloud1").
		that_contains_foo_changes_only_for_directories(
			"/promoted/test/new-test-cluster-1/cloud1",
			"/config/test/new-test-cluster-1/cloud1")

	then.
		a_PR_for("foo", environment.Test, "new-test-cluster-2-cloud2").
		has_labels("k8s-promoter/automated-promotion").
		has_no_assignees().
		has_branch().with_one_commit().
		that_has_kustomization_for_workloads("/promoted/test/new-test-cluster-2/cloud2", "foo").
		that_contains_updated_foo_manifests_for_cluster("/promoted/test/new-test-cluster-2/cloud2").
		that_contains_foo_changes_only_for_directories(
			"/promoted/test/new-test-cluster-2/cloud2",
			"/config/test/new-test-cluster-2/cloud2")
}

func Test_PromotionOfNewTestClustersWithManualManifestsChanges(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		old_dev_manifests_for_the_workload_foo().
		old_test_manifests_for_the_workload_foo().
		new_source_manifests_for_the_workload("foo").
		a_clusters_configuration_file_with_new_test_clusters().
		commit_range_start().
		new_dev_manifests_for_the_workload_foo().
		commit_range_end()

	when.
		promote().
		with_env(environment.Test).
		is_called()

	then.
		promote_succeeds().
		the_remote_repository_is_updated_with_new_branches(5).
		the_number_of_raised_PRs_equals(5)

	// promotions to existing clusters
	then.
		a_PR_for("foo", environment.Test, "test1-cloud1").
		has_labels("k8s-promoter/automated-promotion").
		has_assignees("test-user-2", "test-user-3").
		has_branch().with_one_commit().with_source_commit().
		that_contains_updated_foo_manifests_for_cluster("/promoted/test/test1/cloud1").
		that_contains_changes_only_for_directory("/promoted/test/test1/cloud1").
		that_has_kustomization_for_workloads("/promoted/test/test1/cloud1", "foo")

	then.
		a_PR_for("foo", environment.Test, "test2-cloud1").
		has_labels("k8s-promoter/automated-promotion").
		has_assignees("test-user-2", "test-user-3").
		has_branch().with_one_commit().with_source_commit().
		that_contains_updated_foo_manifests_for_cluster("/promoted/test/test2/cloud1").
		that_contains_changes_only_for_directory("/promoted/test/test2/cloud1").
		that_has_kustomization_for_workloads("/promoted/test/test2/cloud1", "foo")

	then.
		a_PR_for("foo", environment.Test, "test3-cloud2").
		has_labels("k8s-promoter/automated-promotion").
		has_assignees("test-user-2", "test-user-3").
		has_branch().with_one_commit().with_source_commit().
		that_contains_updated_foo_manifests_for_cluster("/promoted/test/test3/cloud2").
		that_contains_changes_only_for_directory("/promoted/test/test3/cloud2").
		that_has_kustomization_for_workloads("/promoted/test/test3/cloud2", "foo")

	// promotions to new clusters
	then.
		a_PR_for("foo", environment.Test, "new-test-cluster-1-cloud1").
		has_labels("k8s-promoter/automated-promotion").
		has_no_assignees().
		has_branch().with_one_commit().
		that_contains_updated_foo_manifests_for_cluster("/promoted/test/new-test-cluster-1/cloud1").
		that_contains_foo_changes_only_for_directories(
			"/promoted/test/new-test-cluster-1/cloud1",
			"/config/test/new-test-cluster-1/cloud1").
		that_has_kustomization_for_workloads("/promoted/test/new-test-cluster-1/cloud1", "foo")

	then.
		a_PR_for("foo", environment.Test, "new-test-cluster-2-cloud2").
		has_labels("k8s-promoter/automated-promotion").
		has_no_assignees().
		has_branch().with_one_commit().
		that_contains_updated_foo_manifests_for_cluster("/promoted/test/new-test-cluster-2/cloud2").
		that_contains_foo_changes_only_for_directories(
			"/promoted/test/new-test-cluster-2/cloud2",
			"/config/test/new-test-cluster-2/cloud2").
		that_has_kustomization_for_workloads("/promoted/test/new-test-cluster-2/cloud2", "foo")
}

func Test_PromotionOfNewTestClusterWhenWorkloadNotPromotedToDevelopment(t *testing.T) {
	given, when, then := PromoteTest(t)

	given.
		a_repository().
		with_config_for_the_workload("foo").
		a_fake_github_server().
		new_source_manifests_for_the_workload("foo").
		commit_range_start().
		a_clusters_configuration_file_with_new_test_clusters().
		commit_range_end()

	when.
		promote().
		with_env(environment.Test).
		is_called()

	then.
		promote_succeeds().
		the_remote_repository_is_not_updated_with_new_branch().
		the_number_of_raised_PRs_equals(0)
}

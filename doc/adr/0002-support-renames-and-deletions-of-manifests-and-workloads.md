# 2. Support renames and deletions of manifests and workloads

Date: 2022-01-20

## Status

Accepted

## Context

Umbrella ticket: <https://github.com/form3tech/tooling-team/issues/390>

lucidchart: <https://lucid.app/lucidchart/3d1a77e0-05dd-4ef6-9594-d976945289f2/edit?from_internal=true>

We want to provide a way for our users to be able to rename workloads, manifests through our promotion tool.

We discussed and evaluated a few approaches:

### Generating patch files and applying them in promoted environments

By using git diff, we could generate patch files to promote changes further.

Example where we rename a manifest file from `csi-driver-helmrelease.yaml` to `helmrelease.yaml` and update the
kustomization.

```diff
git diff origin/master..HEAD > promotion.patch

diff --git a/flux/manifests/csi-driver/csi-driver-helmrelease.yaml b/flux/manifests/csi-driver/helmrelease.yaml
similarity index 100%
rename from flux/manifests/csi-driver/csi-driver-helmrelease.yaml
rename to flux/manifests/csi-driver/helmrelease.yaml
diff --git a/flux/manifests/csi-driver/kustomization.yaml b/flux/manifests/csi-driver/kustomization.yaml
index a07504a..8610c6f 100644
--- a/flux/manifests/csi-driver/kustomization.yaml
+++ b/flux/manifests/csi-driver/kustomization.yaml
@@ -4,4 +4,4 @@ kind: Kustomization
 resources: []
 # Disabled until we run Vault multi-cloud
 #  - csi-driver-helmrepository.yaml
-#  - csi-driver-helmrelease.yaml
+#  - helmrelease.yaml

```

We would be able to take the following patch file, update the paths in it for each targeted promotion environment and
run
`git apply -f promotion.patch`

#### Benefits

We would keep it simple, native tools that everyone understands. Promotion would imply we are moving a `patch` forward
to new environments which captures the actual change itself rather than to copy files from source to target
environments.

#### Drawbacks

- We would have to be using the native git client as go-git doesn't have `apply` functionality.
- The patch apply would need to be aware of workload exclusion, which forces us to make the promotion smarter than we
  would wish with this approach.
  - if we managed to remove workload exclusions and rely instead on kustomization layer, then perhaps it's a viable
      option at some point in the future.

### Generating a `diff` to work out what changed

When a pull request is merged and k8s promoter is run as part of Travis, we have a commit range at our disposal. The
commit range is the set of commits a PR represents.

Change-detector used git diff to work out the workload names to understand what to copy, as it always assumed additions
rather than deletions or renames. We take that approach a bit further and try to leverage git diff to observe what files
have changed in such a commit range.

From the set of diffs we then infer what workloads have been changed or deleted. If a workload doesn't contain any more
manifests, we consider it to be deleted.

### Option 1: Wrapping native git

By using `git diff-index -M50 --name-status X Y`, we could figure out what has changed thus replay the output as an
instruction list.

Example:

```diff
git diff-index -M50 --name-status origin/master .
R100 flux/manifests/csi-driver/csi-driver-helmrelease.yaml flux/manifests/csi-driver/helmrelease.yaml
M flux/manifests/csi-driver/kustomization.yaml
```

#### Option 1 - Benefits

- we wouldn't need to deal with inferring what operations they are as git diff-index would tell us if it's a rename,
  modification or deletion.
- It would be quite native to the tooling we're using and fairly simple concept to follow

#### Option 1 - Drawbacks

- We would be dependent on Git version for the output structure as it's not guaranteed to be stable
- We would have to bundle k8s-promoter in a Docker image with Git instead of shipping a simple binary
- We would have to invest a fair amount of effort in assessing our interpretation of `diff-index` is correct for all
  sorts of expected diffs
- We would have to introduce tar archives of test repositories as you can't nest git repositories.
  - In general the testing approach looked to be uncomfortable due to this aspect.

### Option 2: Using Go-Git go library

Similar approach to above, however, we would be using go-git library which we already are using¸ to diff the two commits
and infer the changes from `object.Changes` structs that are returned.

#### Option 2 - Benefits

- We would keep everything in Go
- It would be easier to handle upgrades
- We would be able to express end-to-end tests without leaving the realms of Go.

#### Option 2 - Drawbacks

- We have to learn Go-Git internals, and it's quite complicated
- We would have to infer the operations from `object.Changes` which only provides From & To entities per change.

## Decision

We decided to go with Go-Git library as we believe it's the solution which will leave us with most flexibility and
resources to build upon the solution for future.

## Consequences

### Positives

- From a users end point the tool will provide a better UX experience as it will do what users expect
- We would learn Go-Git well as it's a library we depend on heavily

### Negatives

- k8s-promoter will become more complex by supporting workload removals, renames as well as manifest renames within a
workload at the same time as we handle workload exclusions.

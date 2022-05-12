# 5. Improve promotion pull requests

Date: 2022-02-14

## Status

Accepted

## Context

In the near future, there will be three types on changes promoted by `k8s-promoter`​:

- changes done by hand to `flux/manifests`
- changes related to new clusters being detected (initial promotion of workloads to the new cluster)
- changes related to new version of helm charts, triggered by push to master by `flux-image-updater` controller

It's unlikely that all three of them happen in the same pull request, but we cannot rule out the following two scenarios:

- manifests are modified by hand and new cluster is detected,
- chart version is bumped and new cluster is detected.

Both will cause the resulting PR to contain more that one could expect, which could lead to some confusion on the reviewers part.

The second aspect is how titles are generated at the moment.
With the growing number of development clusters, since we group all development promotions in a single pull request, the titles have become very verbose. And possibly not very accurate, given that we build the title by looking at all workloads in the commit range and all clusters in the target environment. That can lead to impression that all workloads are promoted to all environments, which is not always true (in case of workloads with workload exclusions).

## Decision

`k8s-promoter` will keep track of workloads and clusters to which they were promoted.
Since we already have a section in PR description where we tag authors and point to commits that are promoted, we will extend it by adding a workload promotion table with workload names and clusters as table axes. `k8s-promoter` will build this table marking each promoted workload in each cluster as such.

We will also change how PR titles are build for `development` clusters. Titles will be generated as follows:

- for `development`: "Promote {workload1}, {workload2} to development"
- for `test/production`: "Promote {workload1}, {workload2} to test ({cluster_name})"

Both pull requests also will contain the promotion table section.

## Consequences

- Pull requests titles for development will become shorter, while those for higher envs will remain distinct
- Promotion table will become the source of truth to find out what has been promoted and where

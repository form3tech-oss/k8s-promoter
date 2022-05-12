# 4. Centralise cluster management

Date: 2022-02-13

## Status

Accepted

## Context

At the moment, `k8s-promoter` is reliant on `clusters.yaml` file being present in each tenant repository, which describes all clusters in all environments.`k8s-promoter` consumes the file to work out which clusters belong to the environment.

This causes a number of inconveniences, namely:

- it is easy for tenant's `cluster.yaml` to go out of sync (not all tenant repositories are updated when new cluster is added)
- `k8s-promoter` does not create directory structure for new cluster when promoting workloads.

The problem is described more in [#699](https://github.com/form3tech/tooling-team/issues/699).

## Decision

When starting promotion, `k8s-promoter` will fetch clusters file from one, central location. We will consider [infrastructure-k8s-admin](https://github.com/form3tech/infrastructure-k8s-admin/) as the source of truth for clusters. To get the file, we'll use <https://docs.github.com/en/rest/reference/repos#get-repository-content>, which allows retrieving a single file from a repository.

`k8s-promoter` will also analyse local directory structure and search for each clusters' `manifestFolder`. If not present, `k8s-promoter` will consider this cluster a new one and promote workloads accordingly.

## Consequences

- 1 additional GitHub API call will be made before promotion starts:
  - the endpoint can only retrieve files up to 1MB (with 20 clusters in `clusters.yaml`​ at the moment the file takes up 3.57KB)
  - GET API calls are not subjected to GitHub's [secondary API limits](https://docs.github.com/en/rest/guides/best-practices-for-integrators#dealing-with-secondary-rate-limits)
- There could be cases where `k8s-promoter`​ detects new cluster when promoting changes done to manifests by hand. This will cause the resulting PR to contain more changes that one can anticipate (i.e workloads promoted to new cluster).

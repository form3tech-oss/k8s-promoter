# 6. Raise separate PRs for initial promotion to new cluster

Date: 2022-02-15

## Status

Accepted

## Context

Given that `k8s-promoter` fetches `clusters.yaml` from one central location, it's plausible that new cluster will be detected when promoting changes introduced to manifests manually by an engineer.
This situation can cause the resulting pull request to be larger than anticipated, which might lead to some confusion.

## Decision

We decided to raise a separate pull request for inital promotion of workloads to newly detected cluster. `k8s-promoter` will be quite verbose about this, logging both the fact that second pull request is raised and why ("new cluster detected").

Additionally, we'll extend the promotion table to mark the new cluster as such.

## Consequences

- On occasion, additional pull request will be raised by `k8s-promoter`
- when new cluster is detected, we'll make 3 additional GitHub API calls: to raise the pull request, add labels, and add assignees
  - however, we wait `1s` before each call according to [best practices](https://docs.github.com/en/rest/guides/best-practices-for-integrators#dealing-with-secondary-rate-limits)
  - adding new cluster should not be that common

# 3. Tag authors in PR bodies

Date: 2022-01-25

## Status

Accepted

## Context

At the moment, when k8s-promoter raises a pull request for promoting a workload change, it is not clear who owns the
change, thus there is a risk we have Pull Requests open for longer than we want.

Currently, we do not have a way to tie a promotion pull request in any environment, to the change that occurred in the
source manifests directory `/flux/manifests/`.

This problem is described more in [#671](https://github.com/form3tech/tooling-team/issues/671).

## Decision

When raising a PR, `k8s-promoter` will list the original source manifest commits and tag the author/committer against
each one. This will ensure that those authors get a notification when a new PR is raised. Additionally, when viewing a
PR, it will be clear who is responsible for it.

## Consequences

If an automated system such as Flux image updater is used to make the source manifest changes in future
(see [#673](https://github.com/form3tech/tooling-team/issues/673)), then the Git author/committer will be a bot. The
mechanism described in this ADR will then fail to find who to tag in the PRs. In that scenario `k8s-promoter` may
have to start tagging the team instead of the authors. [#692](https://github.com/form3tech/tooling-team/issues/692)
has been raised to consider this.

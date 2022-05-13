# k8s-promoter

A CLI tool for promoting k8s manifests that also exposes its functionalities via docker container.

## Usage

Promote changes done between commit `f771f55` and `0def095` in `some-owner/your-tenant` tenant to `test` environment:

```bash
./k8s-promoter \
  --owner some-owner
  --repository your-tenant \
  --target test \
  --commit-range f771f55...0def095 \
  --config-repository your-config-repo \
  --committer-name "Promotion Bot" \
  --committer-email "bot@k8s.promote"
  --gpg-key-path /path/to/promotion-bot-private-key.pem \
```

As the result `k8s-promoter` tool opens a Github PR per cluster to promote the workload.
An exception is made for `development` clusters, as `k8s-promoter` will group all clusters in a single PR.

## Terminology

| Term | Description |
| ---- | ----- |
| **Manifest** | A Kubernetes API object represented in a YAML file. |
| **SourceManifest** | A.k.a "bleeding edge", where we start to apply our changes to our manifests. This will trigger promotion to our environments |
| **Workload** | A unit of work that is represented by a set of manifests. E.g. API-service, Kubernetes Operator, cron job |
| **Environments** | Development, Test, Production, i.e specific environments which we have clusters running. Promotion works in the same order the environments are mentioned |
| **Stack** | A stack is a ring fenced copy of the platform including all of the infrastructure. |
| **Cloud** | Cloud represents which provider we are using for the specific `cluster` |
| **Cluster** | A kubernetes cluster that runs within a stack and cloud vendor. |

## Configuration

The tool expects:

- a clusters configuration file in the config repository (`clusters.yaml` by default).
- an optional `workload.yaml` file in each of the manifest folders (which are themselves subfolders of `/manifests`)

While the structure of these files is deliberately similar to that of Kubernetes CRDs they are *not* run in the Kubernetes clusters.

### `clusters.yaml`

 Describes:

- the layout of the cluster directories within the repository
- which environment each cluster belongs to
- any other metadata which may be used by workloads to selectively target which clusters they are deployed to.

```yaml
version: "v0.1"
configType: Cluster
metadata:
  name: dev1-cloud1
  labels:
    environment: development
    cloud: cloud1
spec:
  manifestFolder: /promoted/development/dev1/cloud1
---
version: "v0.1"
configType: Cluster
metadata:
  name: dev2-cloud2
  labels:
    environment: development
    cloud: cloud2
spec:
  manifestFolder: /promoted/development/dev2/cloud2
```

### `workload.yaml`

Can optionally be specified in the manifest folder and used to specify:

- a set of exclusion rules to target the workload, based on each cluster's labels

```yaml
version: "v0.1"
configType: Workload
metadata:
  name: foo
  description: "A workload which should be applied to cloud2 clusters"
spec:
  exclusions: 
  - key: "cloud"
    operator: "NotEqual"
    value: "cloud2"
```

## Contributing

### Development

This project uses [Magefile](https://magefile.org/) build tool. Follow [installation instructions](https://github.com/magefile/mage#installation) to set it up.

Once installed, run `mage` to list available targets.

### Release

After merging to master, GitHub Action will push new tag, which in turn will trigger `goreleaser` action to build and upload new release.

Please use [conventional commits](https://www.conventionalcommits.org/en/v1.0.0/#summary) as they are used to figure out version bump for tags.

Docs:

- <https://github.com/goreleaser/goreleaser-action>
- <https://github.com/mathieudutour/github-tag-action>

package clusterconf

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"

	"github.com/form3tech/k8s-promoter/internal/environment"

	"gopkg.in/yaml.v3"
)

type Cluster struct {
	Version    string          `yaml:"version"`
	ConfigType string          `yaml:"configType"`
	Metadata   ClusterMetadata `yaml:"metadata"`
	Spec       ClusterSpec     `yaml:"spec"`
}

type ClusterMetadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Labels      Labels `yaml:"labels"`
}

type Labels map[string]string

type ClusterSpec struct {
	ManifestFolder string `yaml:"manifestFolder"`
	ConfigFolder   string `yaml:"configFolder"`
}

// AllowWorkload should pass if there is a zero-value config. This could happen
// if there was no workload config file to be parsed, and this is currently acceptable.
func (c *Cluster) AllowWorkload(wr Workload) bool {
	for _, exc := range wr.Spec.Exclusions {
		if exc.Excludes(c.Metadata.Labels) {
			return false
		}
	}
	return true
}

func (c *Cluster) Name() string {
	return c.Metadata.Name
}

func (c *Cluster) ManifestFolder() string {
	return c.Spec.ManifestFolder
}

func (c *Cluster) ConfigFolder() string {
	return c.Spec.ConfigFolder
}

func (c Cluster) WorkloadPath(name string) string {
	return filepath.Join(c.Spec.ManifestFolder, name)
}

func (c Cluster) validate() error {
	if c.Name() == "" {
		return errors.New("name is required")
	}
	if c.Spec.ManifestFolder == "" {
		return errors.New("manifestfolder is required")
	}
	return environment.Env(c.Metadata.Labels["environment"]).Validate()
}

// FilterByKeys returns a new Labels struct containing only labels with
// keys matching keys argument.
func (l Labels) FilterByKeys(keys ...string) Labels {
	labels := make(Labels)
	for _, k := range keys {
		v, ok := l[k]
		if ok {
			labels[k] = v
		}
	}
	return labels
}

// Keys returns the label keys in alphabetical order.
func (l Labels) Keys() []string {
	var keys []string
	for k := range l {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Hash applies a SHA256 hash to the concatenation of all labels. This is
// useful to uniquely identify a set of labels.
func (l Labels) Hash() string {
	keys := l.Keys()

	hash := sha256.New()
	for _, k := range keys {
		hash.Write([]byte(k + l[k]))
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

type Clusters []Cluster
type FilterFn func(c Cluster) bool

func (c Clusters) Contains(pretender Cluster) bool {
	for _, cluster := range c {
		if cluster.Name() == pretender.Name() {
			return true
		}
	}

	return false
}

func ByEnvironment(environment environment.Env) FilterFn {
	return func(c Cluster) bool {
		env, ok := c.Metadata.Labels["environment"]
		if !ok {
			return false
		}

		return env == string(environment)
	}
}

func ByAllowWorkload(workload Workload) FilterFn {
	return func(c Cluster) bool {
		return c.AllowWorkload(workload)
	}
}

func Without(clusters Clusters) FilterFn {
	return func(c Cluster) bool {
		return !clusters.Contains(c)
	}
}

func (c Clusters) Filter(predicate FilterFn) Clusters {
	var filtered Clusters
	for _, cluster := range c {
		if predicate(cluster) {
			filtered = append(filtered, cluster)
		}
	}

	return filtered
}

// Group groups all development clusters together in one group
// for test & production, we group the clusters individually
func (c Clusters) Group(target environment.Env) []Clusters {
	var res []Clusters

	if target != environment.Development {
		for _, cluster := range c {
			res = append(res, Clusters{cluster})
		}
	}

	if target == environment.Development {
		res = []Clusters{c}
	}
	return res
}

func ParseClusters(in io.Reader) (Clusters, error) {
	decoder := yaml.NewDecoder(in)
	clusters := Clusters{}

	for {
		cluster, err := decodeCluster(decoder)
		if err != nil {
			if err == io.EOF {
				break // We've read everything in the file
			}
			return Clusters{}, fmt.Errorf("could not read config file: %w", err)
		}

		clusters = append(clusters, cluster)
	}
	return clusters, nil
}

func decodeCluster(decoder *yaml.Decoder) (Cluster, error) {
	cluster := Cluster{}
	if err := decoder.Decode(&cluster); err != nil {
		return Cluster{}, err
	}

	if err := cluster.validate(); err != nil {
		return Cluster{}, err
	}
	return cluster, nil
}

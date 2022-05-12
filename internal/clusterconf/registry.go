package clusterconf

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type WorkloadRegistry interface {
	Get(workloadID string) (Workload, error)
	GetAll() ([]Workload, error)
}

type FSWorkloadRegistry struct {
	fs      billy.Filesystem
	rootDir string
	logger  *logrus.Entry
}

func NewWorkloadRegistry(fs billy.Filesystem, rootDir string, log *logrus.Entry) *FSWorkloadRegistry {
	return &FSWorkloadRegistry{
		fs:      fs,
		rootDir: rootDir,
		logger:  log.WithField("module", "WorkloadRegistry"),
	}
}

// Get returns the Workload struct with workloadID set if the file cannot be found
// This means the workload does not have any exclusion rules applied
func (r *FSWorkloadRegistry) Get(workloadID string) (Workload, error) {
	workload, err := r.parseWorkload(workloadID)
	if err == nil {
		return workload, nil
	}

	// For backward compatibility, we won't fail if there is no file
	if os.IsNotExist(err) {
		r.logger.WithError(err).Debug("Workload seems to be missing config")
		return workload, nil
	}

	return workload, fmt.Errorf("error loading workload `%s`: %w", workloadID, err)
}

func (r *FSWorkloadRegistry) GetAll() ([]Workload, error) {
	fileInfos, err := r.fs.ReadDir(r.rootDir)
	if err != nil {
		return nil, err
	}

	var workloads []Workload
	for _, fileInfo := range fileInfos {
		if !fileInfo.IsDir() {
			continue
		}

		workload, err := r.Get(fileInfo.Name())
		if err != nil {
			return nil, err
		}

		workloads = append(workloads, workload)
	}

	return workloads, nil
}

func (r *FSWorkloadRegistry) parseWorkload(workloadID string) (Workload, error) {
	workload := Workload{
		Version:    "v0.1",
		ConfigType: "Workload",
		Metadata: WorkloadMetadata{
			Name: workloadID,
		},
	}

	path := filepath.Join(r.rootDir, workloadID, "workload.yaml")
	f, err := r.fs.Open(path)
	if err != nil {
		return workload, err
	}
	defer func() {
		if err := f.Close(); err != nil {
			r.logger.WithError(err).Error("parseWorkload: f.Close")
		}
	}()

	err = yaml.NewDecoder(f).Decode(&workload)
	if err != nil {
		return workload, fmt.Errorf("could not decode workload config file: %w", err)
	}

	if err := workload.Validate(); err != nil {
		return workload, err
	}

	return workload, nil
}

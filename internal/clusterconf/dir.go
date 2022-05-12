package clusterconf

import "path/filepath"

const TopLevelDir = "/flux"

// Path is a helper function which returns path with a leading top-level directory segment.
func Path(path string) string {
	return filepath.Join(TopLevelDir, path)
}

package version

import (
	"fmt"
	"time"
)

var (
	// Version follows format: vYY.M.PATCH
	// Example: v26.1.0 (year 2026, month 1, patch 0)
	Version = "v26.1.0"

	// BuildTime is set during build
	BuildTime = ""

	// GitCommit is set during build
	GitCommit = ""
)

// Info contains version information
type Info struct {
	Version   string `json:"version"`
	BuildTime string `json:"build_time"`
	GitCommit string `json:"git_commit"`
}

// Get returns version information
func Get() Info {
	buildTime := BuildTime
	if buildTime == "" {
		buildTime = time.Now().Format("2006-01-02 15:04:05")
	}

	return Info{
		Version:   Version,
		BuildTime: buildTime,
		GitCommit: GitCommit,
	}
}

// String returns version as string
func (i Info) String() string {
	if i.GitCommit != "" {
		return fmt.Sprintf("%s (commit: %s, built: %s)", i.Version, i.GitCommit[:7], i.BuildTime)
	}
	return fmt.Sprintf("%s (built: %s)", i.Version, i.BuildTime)
}

// Short returns short version string
func (i Info) Short() string {
	return i.Version
}

package jenkins

import "time"

// Job represents a Jenkins job (can be a regular job or a folder)
type Job struct {
	Name     string `json:"name"`
	FullName string `json:"fullName"`
	URL      string `json:"url"`
	Color    string `json:"color"` // Color indicates status: blue=success, red=failed, yellow=unstable, grey=disabled, etc.

	// LastBuild contains information about the most recent build
	LastBuild *Build `json:"lastBuild"`

	// Jobs is populated if this is a folder containing other jobs
	Jobs []Job `json:"jobs"`

	// Class indicates the type (e.g., "hudson.model.FreeStyleProject", "com.cloudbees.hudson.plugins.folder.Folder")
	Class string `json:"_class"`
}

// Build represents a Jenkins build
type Build struct {
	Number    int    `json:"number"`
	Result    string `json:"result"` // SUCCESS, FAILURE, UNSTABLE, ABORTED, null (building)
	Duration  int64  `json:"duration"` // Duration in milliseconds
	Timestamp int64  `json:"timestamp"` // Unix timestamp in milliseconds
	Building  bool   `json:"building"`
	URL       string `json:"url"`
}

// IsFolder returns true if this job is a folder containing other jobs
func (j *Job) IsFolder() bool {
	return len(j.Jobs) > 0 ||
		j.Class == "com.cloudbees.hudson.plugins.folder.Folder" ||
		j.Class == "org.jenkinsci.plugins.workflow.multibranch.WorkflowMultiBranchProject"
}

// GetStatus returns a normalized status string for display
func (j *Job) GetStatus() string {
	if j.IsFolder() {
		return "FOLDER"
	}

	if j.LastBuild == nil {
		return "NEVER_BUILT"
	}

	if j.LastBuild.Building {
		return "BUILDING"
	}

	// Jenkins uses color codes: blue/blue_anime, red/red_anime, yellow/yellow_anime, grey, disabled, aborted, notbuilt
	switch {
	case j.Color == "blue" || j.Color == "blue_anime":
		return "SUCCESS"
	case j.Color == "red" || j.Color == "red_anime":
		return "FAILED"
	case j.Color == "yellow" || j.Color == "yellow_anime":
		return "UNSTABLE"
	case j.Color == "grey":
		return "PENDING"
	case j.Color == "disabled":
		return "DISABLED"
	case j.Color == "aborted":
		return "ABORTED"
	case j.Color == "notbuilt":
		return "NOT_BUILT"
	default:
		if j.LastBuild.Result != "" {
			return j.LastBuild.Result
		}
		return "UNKNOWN"
	}
}

// GetDuration returns the build duration as a time.Duration
func (b *Build) GetDuration() time.Duration {
	return time.Duration(b.Duration) * time.Millisecond
}

// GetTimestamp returns the build timestamp as a time.Time
func (b *Build) GetTimestamp() time.Time {
	return time.Unix(b.Timestamp/1000, (b.Timestamp%1000)*int64(time.Millisecond))
}

// JobsResponse represents the response from Jenkins API when fetching all jobs
type JobsResponse struct {
	Jobs []Job `json:"jobs"`
}

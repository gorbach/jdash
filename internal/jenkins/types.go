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

// QueueItem represents an item in the Jenkins build queue
type QueueItem struct {
	ID         int    `json:"id"`
	Blocked    bool   `json:"blocked"`
	Buildable  bool   `json:"buildable"`
	Stuck      bool   `json:"stuck"`
	Why        string `json:"why"`        // Reason for being in queue
	InQueueSince int64 `json:"inQueueSince"` // Unix timestamp in milliseconds

	// Task contains job information
	Task struct {
		Name  string `json:"name"`
		URL   string `json:"url"`
		Color string `json:"color"`
	} `json:"task"`

	// Executable contains build information if the item is currently building
	Executable *struct {
		Number int    `json:"number"`
		URL    string `json:"url"`
	} `json:"executable"`
}

// QueueResponse represents the response from Jenkins queue API
type QueueResponse struct {
	Items []QueueItem `json:"items"`
}

// IsBuilding returns true if this queue item is currently executing
func (q *QueueItem) IsBuilding() bool {
	return q.Executable != nil
}

// GetJobName returns the job name from the task
func (q *QueueItem) GetJobName() string {
	return q.Task.Name
}

// GetBuildNumber returns the build number if building, otherwise 0
func (q *QueueItem) GetBuildNumber() int {
	if q.Executable != nil {
		return q.Executable.Number
	}
	return 0
}

// GetInQueueDuration returns how long this item has been in queue
func (q *QueueItem) GetInQueueDuration() time.Duration {
	now := time.Now().UnixMilli()
	return time.Duration(now-q.InQueueSince) * time.Millisecond
}

// Executor represents a Jenkins executor (build slot)
type Executor struct {
	Idle             bool `json:"idle"`
	CurrentExecutable *struct {
		FullDisplayName string `json:"fullDisplayName"`
		Number          int    `json:"number"`
		URL             string `json:"url"`
		Timestamp       int64  `json:"timestamp"` // Unix timestamp in milliseconds
	} `json:"currentExecutable"`
}

// Computer represents a Jenkins node (master or agent)
type Computer struct {
	DisplayName string     `json:"displayName"`
	Executors   []Executor `json:"executors"`
}

// ComputerResponse represents the response from Jenkins computer API
type ComputerResponse struct {
	Computer []Computer `json:"computer"`
}

// RunningBuild represents a build currently executing on an executor
type RunningBuild struct {
	JobName     string
	BuildNumber int
	StartTime   int64  // Unix timestamp in milliseconds
	URL         string
	Node        string
}

// GetElapsedTime returns how long this build has been running
func (r *RunningBuild) GetElapsedTime() time.Duration {
	now := time.Now().UnixMilli()
	return time.Duration(now-r.StartTime) * time.Millisecond
}

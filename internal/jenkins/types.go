package jenkins

import (
	"fmt"
	"strings"
	"time"
)

// Job represents a Jenkins job (can be a regular job or a folder)
type Job struct {
	Name        string `json:"name"`
	FullName    string `json:"fullName"`
	URL         string `json:"url"`
	Color       string `json:"color"` // Color indicates status: blue=success, red=failed, yellow=unstable, grey=disabled, etc.
	Description string `json:"description"`

	// LastBuild contains information about the most recent build
	LastBuild *Build `json:"lastBuild"`

	// Jobs is populated if this is a folder containing other jobs
	Jobs []Job `json:"jobs"`

	// Class indicates the type (e.g., "hudson.model.FreeStyleProject", "com.cloudbees.hudson.plugins.folder.Folder")
	Class string `json:"_class"`
}

// Build represents a Jenkins build
type Build struct {
	Number    int           `json:"number"`
	Result    string        `json:"result"`    // SUCCESS, FAILURE, UNSTABLE, ABORTED, null (building)
	Duration  int64         `json:"duration"`  // Duration in milliseconds
	Timestamp int64         `json:"timestamp"` // Unix timestamp in milliseconds
	Building  bool          `json:"building"`
	URL       string        `json:"url"`
	Actions   []BuildAction `json:"actions"`
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
		return StatusFolder
	}

	if j.LastBuild == nil {
		return StatusNeverBuilt
	}

	if j.LastBuild.Building {
		return StatusBuilding
	}

	// Jenkins uses color codes: blue/blue_anime, red/red_anime, yellow/yellow_anime, grey, disabled, aborted, notbuilt
	switch {
	case j.Color == "blue" || j.Color == "blue_anime":
		return StatusSuccess
	case j.Color == "red" || j.Color == "red_anime":
		return StatusFailed
	case j.Color == "yellow" || j.Color == "yellow_anime":
		return StatusUnstable
	case j.Color == "grey":
		return StatusPending
	case j.Color == "disabled":
		return StatusDisabled
	case j.Color == "aborted":
		return StatusAborted
	case j.Color == "notbuilt":
		return StatusNotBuilt
	default:
		if j.LastBuild.Result != "" {
			return j.LastBuild.Result
		}
		return StatusUnknown
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

// GetStatus returns a normalized status string for display
func (b *Build) GetStatus() string {
	if b == nil {
		return ""
	}
	if b.Building {
		return StatusBuilding
	}
	if b.Result == "" {
		return StatusUnknown
	}
	return strings.ToUpper(b.Result)
}

// GetTriggeredBy attempts to extract the triggering user or cause from build actions.
func (b *Build) GetTriggeredBy() string {
	if b == nil {
		return ""
	}
	for _, action := range b.Actions {
		for _, cause := range action.Causes {
			if cause.UserName != "" {
				return cause.UserName
			}
			if cause.UserID != "" {
				return cause.UserID
			}
			if cause.ShortDescription != "" {
				return cause.ShortDescription
			}
		}
	}
	return ""
}

// GetBranch tries to determine the source branch from build actions.
func (b *Build) GetBranch() string {
	if b == nil {
		return ""
	}

	for _, action := range b.Actions {
		if action.LastBuiltRevision != nil {
			for _, branch := range action.LastBuiltRevision.Branches {
				if branch.Name != "" {
					return branch.Name
				}
			}
		}
		for _, param := range action.Parameters {
			if strings.EqualFold(param.Name, "branch") ||
				strings.EqualFold(param.Name, "branch_name") ||
				strings.EqualFold(param.Name, "git_branch") {
				return fmt.Sprint(param.Value)
			}
		}
	}

	return ""
}

// BuildAction represents additional metadata attached to a build.
type BuildAction struct {
	Class             string           `json:"_class"`
	Causes            []BuildCause     `json:"causes"`
	Parameters        []BuildParameter `json:"parameters"`
	LastBuiltRevision *BuildRevision   `json:"lastBuiltRevision"`
}

// BuildCause describes what triggered a build.
type BuildCause struct {
	ShortDescription string `json:"shortDescription"`
	UserID           string `json:"userId"`
	UserName         string `json:"userName"`
}

// BuildParameter represents a parameter passed to the build.
type BuildParameter struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

// BuildRevision contains revision information for SCM-based jobs.
type BuildRevision struct {
	Branches []BuildBranch `json:"branch"`
}

// BuildBranch represents a single SCM branch reference.
type BuildBranch struct {
	SHA1 string `json:"SHA1"`
	Name string `json:"name"`
}

// JobDetails provides expanded information about a Jenkins job.
type JobDetails struct {
	Job
	Builds               []Build               `json:"builds"`
	ParameterDefinitions []ParameterDefinition `json:"-"`
}

// ParameterDefinition describes a parameter configured on a Jenkins job.
type ParameterDefinition struct {
	Class                string                   `json:"_class"`
	Name                 string                   `json:"name"`
	Type                 string                   `json:"type"`
	Description          string                   `json:"description"`
	DefaultParameter     *ParameterDefaultValue   `json:"defaultParameterValue"`
	Choices              []string                 `json:"choices"`
	Trim                 bool                     `json:"trim"`
	BooleanDefault       bool                     `json:"defaultValue"`
	ProjectName          string                   `json:"projectName"`
	ReferencedParameters []ReferencedParameterRef `json:"referencedParameters"`
}

// ParameterDefaultValue represents the default value object for parameter definitions.
type ParameterDefaultValue struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

// ReferencedParameterRef represents a reference for copy parameter definitions.
type ReferencedParameterRef struct {
	Name string `json:"name"`
}

// GetType returns a simplified type string for the parameter definition.
func (p ParameterDefinition) GetType() string {
	if p.Type != "" {
		return p.Type
	}
	if p.Class != "" {
		return p.Class
	}
	return ""
}

// DefaultValueString renders the default value as a string.
func (p ParameterDefinition) DefaultValueString() string {
	if p.DefaultParameter != nil && p.DefaultParameter.Value != nil {
		return fmt.Sprint(p.DefaultParameter.Value)
	}
	switch strings.ToLower(p.GetType()) {
	case "booleanparameterdefinition", "hudson.model.booleanparameterdefinition":
		if p.BooleanDefault {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

// JobsResponse represents the response from Jenkins API when fetching all jobs
type JobsResponse struct {
	Jobs []Job `json:"jobs"`
}

// QueueItem represents an item in the Jenkins build queue
type QueueItem struct {
	ID           int    `json:"id"`
	Blocked      bool   `json:"blocked"`
	Buildable    bool   `json:"buildable"`
	Stuck        bool   `json:"stuck"`
	Why          string `json:"why"`          // Reason for being in queue
	InQueueSince int64  `json:"inQueueSince"` // Unix timestamp in milliseconds

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
	Idle              bool `json:"idle"`
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
	StartTime   int64 // Unix timestamp in milliseconds
	URL         string
	Node        string
}

// GetElapsedTime returns how long this build has been running
func (r *RunningBuild) GetElapsedTime() time.Duration {
	now := time.Now().UnixMilli()
	return time.Duration(now-r.StartTime) * time.Millisecond
}

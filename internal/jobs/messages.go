package jobs

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorbach/jdash/internal/jenkins"
)

// JobsFetchedMsg is sent when jobs have been successfully fetched from Jenkins
type JobsFetchedMsg struct {
	Jobs []jenkins.Job
}

// JobsErrorMsg is sent when there's an error fetching jobs
type JobsErrorMsg struct {
	Err error
}

// JobSelectedMsg notifies other panels that a job was selected.
type JobSelectedMsg struct {
	Job jenkins.Job
}

// JobSelectionClearedMsg indicates that no job is currently selected.
type JobSelectionClearedMsg struct{}

// RefreshRequestedMsg asks the jobs panel to refetch jobs from Jenkins.
type RefreshRequestedMsg struct{}

// fetchJobsCmd creates a command to fetch all jobs from Jenkins
func fetchJobsCmd(client jenkins.JenkinsClient) tea.Cmd {
	return func() tea.Msg {
		jobs, err := client.GetAllJobs()
		if err != nil {
			return JobsErrorMsg{Err: err}
		}
		return JobsFetchedMsg{Jobs: jobs}
	}
}

// jobSelectedCmd returns a command that emits a JobSelectedMsg.
func jobSelectedCmd(job jenkins.Job) tea.Cmd {
	jobCopy := job
	return func() tea.Msg {
		return JobSelectedMsg{Job: jobCopy}
	}
}

// jobSelectionClearedCmd returns a command that emits a JobSelectionClearedMsg.
func jobSelectionClearedCmd() tea.Cmd {
	return func() tea.Msg {
		return JobSelectionClearedMsg{}
	}
}

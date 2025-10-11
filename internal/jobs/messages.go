package jobs

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorbach/jenkins-gotui/internal/jenkins"
)

// JobsFetchedMsg is sent when jobs have been successfully fetched from Jenkins
type JobsFetchedMsg struct {
	Jobs []jenkins.Job
}

// JobsErrorMsg is sent when there's an error fetching jobs
type JobsErrorMsg struct {
	Err error
}

// fetchJobsCmd creates a command to fetch all jobs from Jenkins
func fetchJobsCmd(client *jenkins.Client) tea.Cmd {
	return func() tea.Msg {
		jobs, err := client.GetAllJobs()
		if err != nil {
			return JobsErrorMsg{Err: err}
		}
		return JobsFetchedMsg{Jobs: jobs}
	}
}

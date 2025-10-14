package details

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorbach/jenkins-gotui/internal/jenkins"
)

type actionKind string

const (
	actionKindTriggerBuild   actionKind = "trigger_build"
	actionKindAbortBuild     actionKind = "abort_build"
	actionKindRefresh        actionKind = "refresh"
	actionKindViewLogs       actionKind = "view_logs"
	actionKindViewParameters actionKind = "view_parameters"
	actionKindViewHistory    actionKind = "view_history"
	actionKindViewConfig     actionKind = "view_config"
)

type actionResultMsg struct {
	ticket  uint64
	kind    actionKind
	message string
	err     error
}

type actionMessageClearedMsg struct {
	ticket uint64
}

type actionRequestMsg struct {
	Kind  actionKind
	Job   jenkins.Job
	Build *jenkins.Build
}

const actionFeedbackDuration = 3 * time.Second

func triggerBuildCmd(client *jenkins.Client, jobName, jobFullName string, ticket uint64) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return actionResultMsg{
				ticket: ticket,
				kind:   actionKindTriggerBuild,
				err:    fmt.Errorf("Jenkins client not configured"),
			}
		}

		if err := client.TriggerBuild(jobFullName); err != nil {
			return actionResultMsg{
				ticket: ticket,
				kind:   actionKindTriggerBuild,
				err:    err,
			}
		}

		return actionResultMsg{
			ticket:  ticket,
			kind:    actionKindTriggerBuild,
			message: fmt.Sprintf("✓ Build triggered for %s", jobName),
		}
	}
}

func abortBuildCmd(client *jenkins.Client, jobName, jobFullName string, buildNumber int, ticket uint64) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return actionResultMsg{
				ticket: ticket,
				kind:   actionKindAbortBuild,
				err:    fmt.Errorf("Jenkins client not configured"),
			}
		}
		if buildNumber <= 0 {
			return actionResultMsg{
				ticket: ticket,
				kind:   actionKindAbortBuild,
				err:    fmt.Errorf("no running build to abort"),
			}
		}

		if err := client.AbortBuild(jobFullName, buildNumber); err != nil {
			return actionResultMsg{
				ticket: ticket,
				kind:   actionKindAbortBuild,
				err:    err,
			}
		}

		return actionResultMsg{
			ticket:  ticket,
			kind:    actionKindAbortBuild,
			message: fmt.Sprintf("✓ Abort signal sent to %s (#%d)", jobName, buildNumber),
		}
	}
}

func actionRequestCmd(kind actionKind, job jenkins.Job, build *jenkins.Build) tea.Cmd {
	jobCopy := job
	var buildCopy *jenkins.Build
	if build != nil {
		tmp := *build
		buildCopy = &tmp
	}

	return func() tea.Msg {
		return actionRequestMsg{
			Kind:  kind,
			Job:   jobCopy,
			Build: buildCopy,
		}
	}
}

func clearActionMessageCmd(ticket uint64) tea.Cmd {
	if ticket == 0 {
		return nil
	}
	return tea.Tick(actionFeedbackDuration, func(time.Time) tea.Msg {
		return actionMessageClearedMsg{ticket: ticket}
	})
}

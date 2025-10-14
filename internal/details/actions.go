package details

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorbach/jenkins-gotui/internal/jenkins"
)

type ActionKind string

const (
	ActionKindTriggerBuild           ActionKind = "trigger_build"
	ActionKindTriggerBuildWithParams ActionKind = "trigger_build_with_parameters"
	ActionKindAbortBuild             ActionKind = "abort_build"
	ActionKindRefresh                ActionKind = "refresh"
	ActionKindViewLogs               ActionKind = "view_logs"
	ActionKindViewParameters         ActionKind = "view_parameters"
	ActionKindViewHistory            ActionKind = "view_history"
	ActionKindViewConfig             ActionKind = "view_config"
)

type actionResultMsg struct {
	ticket  uint64
	kind    ActionKind
	message string
	err     error
}

type actionMessageClearedMsg struct {
	ticket uint64
}

type ActionRequestMsg struct {
	Kind                 ActionKind
	Job                  jenkins.Job
	Build                *jenkins.Build
	ParameterDefinitions []jenkins.ParameterDefinition
}

// ParameterSubmissionMsg is sent when a parameter modal submits values.
type ParameterSubmissionMsg struct {
	JobFullName string
	JobName     string
	Values      map[string]string
}

// ParameterCancelledMsg indicates that the parameter collection modal was cancelled.
type ParameterCancelledMsg struct {
	JobFullName string
}

const actionFeedbackDuration = 3 * time.Second

func triggerBuildCmd(client *jenkins.Client, jobName, jobFullName string, ticket uint64) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return actionResultMsg{
				ticket: ticket,
				kind:   ActionKindTriggerBuild,
				err:    fmt.Errorf("Jenkins client not configured"),
			}
		}

		if err := client.TriggerBuild(jobFullName); err != nil {
			return actionResultMsg{
				ticket: ticket,
				kind:   ActionKindTriggerBuild,
				err:    err,
			}
		}

		return actionResultMsg{
			ticket:  ticket,
			kind:    ActionKindTriggerBuild,
			message: fmt.Sprintf("✓ Build triggered for %s", jobName),
		}
	}
}

func abortBuildCmd(client *jenkins.Client, jobName, jobFullName string, buildNumber int, ticket uint64) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return actionResultMsg{
				ticket: ticket,
				kind:   ActionKindAbortBuild,
				err:    fmt.Errorf("Jenkins client not configured"),
			}
		}
		if buildNumber <= 0 {
			return actionResultMsg{
				ticket: ticket,
				kind:   ActionKindAbortBuild,
				err:    fmt.Errorf("no running build to abort"),
			}
		}

		if err := client.AbortBuild(jobFullName, buildNumber); err != nil {
			return actionResultMsg{
				ticket: ticket,
				kind:   ActionKindAbortBuild,
				err:    err,
			}
		}

		return actionResultMsg{
			ticket:  ticket,
			kind:    ActionKindAbortBuild,
			message: fmt.Sprintf("✓ Abort signal sent to %s (#%d)", jobName, buildNumber),
		}
	}
}

func triggerBuildWithParamsCmd(client *jenkins.Client, jobName, jobFullName string, values map[string]string, ticket uint64) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return actionResultMsg{
				ticket: ticket,
				kind:   ActionKindTriggerBuildWithParams,
				err:    fmt.Errorf("Jenkins client not configured"),
			}
		}
		if err := client.TriggerBuildWithParameters(jobFullName, values); err != nil {
			return actionResultMsg{
				ticket: ticket,
				kind:   ActionKindTriggerBuildWithParams,
				err:    err,
			}
		}
		return actionResultMsg{
			ticket:  ticket,
			kind:    ActionKindTriggerBuildWithParams,
			message: fmt.Sprintf("✓ Build triggered for %s", jobName),
		}
	}
}

func actionRequestCmd(kind ActionKind, job jenkins.Job, build *jenkins.Build, params []jenkins.ParameterDefinition) tea.Cmd {
	jobCopy := job
	var buildCopy *jenkins.Build
	if build != nil {
		tmp := *build
		buildCopy = &tmp
	}
	var paramCopy []jenkins.ParameterDefinition
	if len(params) > 0 {
		paramCopy = append([]jenkins.ParameterDefinition(nil), params...)
	}

	return func() tea.Msg {
		return ActionRequestMsg{
			Kind:                 kind,
			Job:                  jobCopy,
			Build:                buildCopy,
			ParameterDefinitions: paramCopy,
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

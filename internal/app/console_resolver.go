package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorbach/jdash/internal/console"
	"github.com/gorbach/jdash/internal/jenkins"
)

type consoleTargetResolvedMsg struct {
	JobFullName string
	BuildNumber int
	BuildURL    string
	Err         error
}

type consoleTargetTracker struct {
	jobFullName string
	jobName     string
	buildURL    string
	buildNumber int
}

func (t consoleTargetTracker) Reset() consoleTargetTracker {
	t.jobFullName = ""
	t.jobName = ""
	t.buildURL = ""
	t.buildNumber = 0
	return t
}

func (t consoleTargetTracker) WithTarget(jobFullName, jobName, buildURL string, buildNumber int) consoleTargetTracker {
	t.jobFullName = jobFullName
	t.jobName = jobName
	t.buildURL = buildURL
	t.buildNumber = buildNumber
	return t
}

func (t consoleTargetTracker) JobFullName() string {
	return t.jobFullName
}

func (t consoleTargetTracker) ApplyResolution(msg consoleTargetResolvedMsg) (consoleTargetTracker, *console.OpenRequestMsg) {
	if msg.Err != nil || msg.JobFullName == "" || msg.JobFullName != t.jobFullName {
		return t, nil
	}

	number := msg.BuildNumber
	url := strings.TrimSpace(msg.BuildURL)

	if number <= 0 {
		number = t.buildNumber
	}
	if url == "" {
		url = t.buildURL
	}

	if number == t.buildNumber && url == t.buildURL {
		return t, nil
	}

	t.buildNumber = number
	t.buildURL = url

	open := console.OpenRequestMsg{
		JobName:     t.jobName,
		JobFullName: t.jobFullName,
		BuildNumber: number,
		BuildURL:    url,
	}

	return t, &open
}

func resolveConsoleTargetCmd(client jenkins.JenkinsClient, jobFullName string) tea.Cmd {
	if client == nil || jobFullName == "" {
		return nil
	}

	return func() tea.Msg {
		build, err := client.GetBuild(jobFullName, -1)
		if err != nil {
			return consoleTargetResolvedMsg{
				JobFullName: jobFullName,
				Err:         err,
			}
		}

		msg := consoleTargetResolvedMsg{
			JobFullName: jobFullName,
		}

		if build != nil {
			msg.BuildNumber = build.Number
			msg.BuildURL = build.URL
		}

		return msg
	}
}

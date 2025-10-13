package queue

import (
	"time"

	"github.com/gorbach/jenkins-gotui/internal/jenkins"
)

// tickMsg is sent every second to update elapsed times
type tickMsg time.Time

// pollQueueMsg triggers a poll of the Jenkins build queue
type pollQueueMsg struct{}

// queueUpdateMsg contains the fetched queue data
type queueUpdateMsg struct {
	queuedItems  []jenkins.QueueItem
	runningBuilds []jenkins.RunningBuild
}

// queueErrorMsg contains error information from queue polling
type queueErrorMsg struct {
	err error
}

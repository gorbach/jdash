package jobs

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"
)

const searchDebounceInterval = 120 * time.Millisecond

type searchQueuedMsg struct {
	Query  string
	Ticket uint64
}

func debounceSearchCmd(query string, ticket uint64) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(searchDebounceInterval)
		return searchQueuedMsg{
			Query:  strings.TrimSpace(query),
			Ticket: ticket,
		}
	}
}

type matchResult struct {
	node    *JobTree
	indexes []int
}

type jobNodeSource struct {
	nodes []*JobTree
	lower []string
}

func newJobNodeSource(nodes []*JobTree) jobNodeSource {
	lower := make([]string, len(nodes))
	for i, node := range nodes {
		lower[i] = strings.ToLower(node.FullName)
	}
	return jobNodeSource{nodes: nodes, lower: lower}
}

func (s jobNodeSource) Len() int {
	return len(s.nodes)
}

func (s jobNodeSource) String(i int) string {
	return s.lower[i]
}

func runFuzzySearch(query string, nodes []*JobTree) []matchResult {
	if query == "" || len(nodes) == 0 {
		return nil
	}

	source := newJobNodeSource(nodes)
	matches := fuzzy.FindFrom(strings.ToLower(query), source)

	results := make([]matchResult, len(matches))
	for i, match := range matches {
		node := source.nodes[match.Index]
		results[i] = matchResult{
			node:    node,
			indexes: append([]int(nil), match.MatchedIndexes...),
		}
	}

	return results
}

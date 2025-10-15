package jobs

import (
	"strings"

	"github.com/gorbach/jdash/internal/jenkins"
)

// JobTree represents a node in the hierarchical job tree
type JobTree struct {
	Name         string       // Display name (not full path)
	FullName     string       // Full path (e.g., "Production/Backend/api-service")
	IsFolder     bool         // True if this is a folder containing other jobs
	Expanded     bool         // True if folder is expanded (children visible)
	Children     []*JobTree   // Child nodes (jobs or folders)
	Job          *jenkins.Job // Actual job data (nil for folders without job data)
	Level        int          // Nesting level (0 = root)
	Parent       *JobTree     // Parent reference (nil for root)
	MatchIndexes []int        // Rune indexes of fuzzy match for highlighting
	SearchResult bool         // True when node is part of current search results
}

// FilterValue implements list.Item interface for bubbles/list filtering
func (j JobTree) FilterValue() string {
	return j.FullName
}

// buildTree converts a flat list of jobs into a hierarchical tree structure
func buildTree(jobs []jenkins.Job) *JobTree {
	// Create root node
	root := &JobTree{
		Name:     "",
		IsFolder: true,
		Expanded: true,
		Children: []*JobTree{},
		Level:    -1,
	}

	// Build tree recursively
	for _, job := range jobs {
		addJobToTree(root, job, 0)
	}

	return root
}

// addJobToTree recursively adds a job to the tree
func addJobToTree(parent *JobTree, job jenkins.Job, level int) {
	node := &JobTree{
		Name:     job.Name,
		FullName: job.FullName,
		IsFolder: job.IsFolder(),
		Expanded: false, // Initially collapsed (except root)
		Children: []*JobTree{},
		Job:      &job,
		Level:    level,
		Parent:   parent,
	}

	// Add nested jobs if this is a folder
	if node.IsFolder && len(job.Jobs) > 0 {
		for _, childJob := range job.Jobs {
			addJobToTree(node, childJob, level+1)
		}
	}

	parent.Children = append(parent.Children, node)
}

// flattenVisibleNodes returns a flat list of visible nodes (respecting expand/collapse state)
func flattenVisibleNodes(tree *JobTree) []*JobTree {
	if tree == nil {
		return []*JobTree{}
	}

	result := []*JobTree{}

	// Don't include root node in the result
	if tree.Level >= 0 {
		result = append(result, tree)
	}

	// If expanded (or is root), include children
	if tree.Expanded || tree.Level < 0 {
		for _, child := range tree.Children {
			result = append(result, flattenVisibleNodes(child)...)
		}
	}

	return result
}

// findNode finds a node in the tree by index in the flattened list
func findNode(tree *JobTree, index int) *JobTree {
	nodes := flattenVisibleNodes(tree)
	if index < 0 || index >= len(nodes) {
		return nil
	}
	return nodes[index]
}

// toggleExpand toggles the expanded state of a node
func toggleExpand(node *JobTree) {
	if node != nil && node.IsFolder {
		node.Expanded = !node.Expanded
	}
}

// expandNode expands a folder node
func expandNode(node *JobTree) {
	if node != nil && node.IsFolder {
		node.Expanded = true
	}
}

// collapseNode collapses a folder node
func collapseNode(node *JobTree) {
	if node != nil && node.IsFolder {
		node.Expanded = false
	}
}

// expandAll recursively expands all folder nodes
func expandAll(tree *JobTree) {
	if tree == nil {
		return
	}

	if tree.IsFolder {
		tree.Expanded = true
	}

	for _, child := range tree.Children {
		expandAll(child)
	}
}

// collapseAll recursively collapses all folder nodes
func collapseAll(tree *JobTree) {
	if tree == nil {
		return
	}

	if tree.IsFolder {
		tree.Expanded = false
	}

	for _, child := range tree.Children {
		collapseAll(child)
	}
}

// getIndentation returns the indentation string for a node based on its level
func getIndentation(level int) string {
	if level <= 0 {
		return ""
	}
	return strings.Repeat("  ", level)
}

// countVisibleNodes returns the number of visible nodes in the tree
func countVisibleNodes(tree *JobTree) int {
	return len(flattenVisibleNodes(tree))
}

// getTotalJobCount returns the total number of jobs (non-folders) in the tree
func getTotalJobCount(tree *JobTree) int {
	if tree == nil {
		return 0
	}

	count := 0
	if !tree.IsFolder && tree.Level >= 0 {
		count = 1
	}

	for _, child := range tree.Children {
		count += getTotalJobCount(child)
	}

	return count
}

// collectAllNodes returns all tree nodes excluding the synthetic root.
func collectAllNodes(tree *JobTree) []*JobTree {
	if tree == nil {
		return nil
	}

	var nodes []*JobTree

	var walk func(node *JobTree)
	walk = func(node *JobTree) {
		if node.Level >= 0 {
			nodes = append(nodes, node)
		}
		for _, child := range node.Children {
			walk(child)
		}
	}

	walk(tree)
	return nodes
}

// clearMatchHighlights removes search-related state from the tree.
func clearMatchHighlights(tree *JobTree) {
	if tree == nil {
		return
	}

	tree.MatchIndexes = nil
	tree.SearchResult = false
	for _, child := range tree.Children {
		clearMatchHighlights(child)
	}
}

// expandPathToNode ensures all ancestors of the node are expanded so the node is visible.
func expandPathToNode(node *JobTree) {
	for current := node; current != nil; current = current.Parent {
		if current.IsFolder {
			current.Expanded = true
		}
	}
}

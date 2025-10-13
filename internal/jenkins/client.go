package jenkins

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client represents a Jenkins API client
type Client struct {
	BaseURL    string
	Username   string
	Token      string
	HTTPClient *http.Client
}

// Credentials holds Jenkins authentication information
type Credentials struct {
	URL      string
	Username string
	Token    string
}

// NewClient creates a new Jenkins client
func NewClient(creds Credentials) *Client {
	return &Client{
		BaseURL:  creds.URL,
		Username: creds.Username,
		Token:    creds.Token,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// doRequest performs an HTTP request with basic auth
func (c *Client) doRequest(method, path string, body io.Reader) (*http.Response, error) {
	url := c.BaseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	// Set basic auth
	req.SetBasicAuth(c.Username, c.Token)
	req.Header.Set("Content-Type", "application/json")

	return c.HTTPClient.Do(req)
}

// TestConnection tests the connection to Jenkins server
// Returns nil if successful, error otherwise
func (c *Client) TestConnection() error {
	resp, err := c.doRequest("GET", "/api/json", nil)
	if err != nil {
		// Check for common network errors
		if err, ok := err.(interface{ Timeout() bool }); ok && err.Timeout() {
			return fmt.Errorf("connection timeout. Jenkins server is not responding")
		}
		return fmt.Errorf("cannot connect to Jenkins server. Please check the URL")
	}
	defer resp.Body.Close()

	// Check response status
	switch resp.StatusCode {
	case http.StatusOK:
		// Try to parse response to validate it's actually Jenkins
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("invalid response from server. Is this a Jenkins instance?")
		}
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("authentication failed. Please check your username and token")
	case http.StatusNotFound:
		return fmt.Errorf("Jenkins API not found. Please check the URL")
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("connection failed: %s (status: %d)", string(body), resp.StatusCode)
	}
}

// GetInfo gets basic Jenkins information
func (c *Client) GetInfo() (map[string]interface{}, error) {
	resp, err := c.doRequest("GET", "/api/json", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get Jenkins info: status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// GetAllJobs fetches all jobs from Jenkins, including nested jobs in folders
// Uses the tree parameter to efficiently fetch nested structures in a single request
func (c *Client) GetAllJobs() ([]Job, error) {
	// Use tree parameter to fetch nested job structure efficiently
	// This fetches job name, fullName, url, color, lastBuild details, and nested jobs
	path := "/api/json?tree=jobs[name,fullName,url,color,_class,lastBuild[number,result,duration,timestamp,building,url],jobs[name,fullName,url,color,_class,lastBuild[number,result,duration,timestamp,building,url],jobs[name,fullName,url,color,_class,lastBuild[number,result,duration,timestamp,building,url]]]]"

	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch jobs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch jobs: status %d, body: %s", resp.StatusCode, string(body))
	}

	var response JobsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode jobs response: %w", err)
	}

	return response.Jobs, nil
}

// GetBuildQueue fetches the current build queue from Jenkins
// This includes both items waiting in queue and items currently executing
func (c *Client) GetBuildQueue() ([]QueueItem, error) {
	// Fetch queue with tree parameter to get all necessary fields
	path := "/queue/api/json?tree=items[id,blocked,buildable,stuck,why,inQueueSince,task[name,url,color],executable[number,url]]"

	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch build queue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch build queue: status %d, body: %s", resp.StatusCode, string(body))
	}

	var response QueueResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode queue response: %w", err)
	}

	return response.Items, nil
}

// GetRunningBuilds fetches currently executing builds from all Jenkins executors
// This checks all nodes (master and agents) and their executors
func (c *Client) GetRunningBuilds() ([]RunningBuild, error) {
	// Fetch computer information with executor details
	path := "/computer/api/json?tree=computer[displayName,executors[idle,currentExecutable[fullDisplayName,number,url,timestamp]]]"

	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch running builds: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch running builds: status %d, body: %s", resp.StatusCode, string(body))
	}

	var response ComputerResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode computer response: %w", err)
	}

	var builds []RunningBuild

	// Loop through all nodes (computers)
	for _, node := range response.Computer {
		// Loop through all executors on this node
		for _, executor := range node.Executors {
			// Skip idle executors
			if executor.Idle || executor.CurrentExecutable == nil {
				continue
			}

			builds = append(builds, RunningBuild{
				JobName:     executor.CurrentExecutable.FullDisplayName,
				BuildNumber: executor.CurrentExecutable.Number,
				StartTime:   executor.CurrentExecutable.Timestamp,
				URL:         executor.CurrentExecutable.URL,
				Node:        node.DisplayName,
			})
		}
	}

	return builds, nil
}

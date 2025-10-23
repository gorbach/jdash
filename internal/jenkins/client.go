package jenkins

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// JenkinsClient defines the interface for interacting with Jenkins API
type JenkinsClient interface {
	// TestConnection tests the connection to Jenkins server
	TestConnection() error

	// GetAllJobs fetches all jobs from Jenkins, including nested jobs in folders
	GetAllJobs() ([]Job, error)

	// GetJobDetails fetches detailed information about a specific job, including recent builds
	GetJobDetails(fullName string, limit int) (*JobDetails, error)

	// GetBuildQueue fetches the current build queue from Jenkins
	GetBuildQueue() ([]QueueItem, error)

	// GetRunningBuilds fetches currently executing builds from all Jenkins executors
	GetRunningBuilds() ([]RunningBuild, error)

	// TriggerBuild requests a new build for the specified job
	TriggerBuild(fullName string) error

	// TriggerBuildWithParameters requests a new build providing parameter values
	TriggerBuildWithParameters(fullName string, params map[string]string) error

	// AbortBuild sends a stop signal to a running build
	AbortBuild(fullName string, buildNumber int) error

	// GetBuild fetches build details for the given job
	GetBuild(fullName string, number int) (*Build, error)

	// GetProgressiveLog fetches a chunk of console output using Jenkins' progressive log API
	GetProgressiveLog(buildURL, fullName string, buildNumber int, start int64) (string, int64, bool, error)
}

// Client represents a Jenkins API client
type Client struct {
	BaseURL    string
	Username   string
	Token      string
	HTTPClient *http.Client

	crumb         *Crumb
	crumbDisabled bool
	crumbMu       sync.Mutex
}

// Credentials holds Jenkins authentication information
type Credentials struct {
	URL      string
	Username string
	Token    string
}

// NewClient creates a new Jenkins client
func NewClient(creds Credentials) JenkinsClient {
	return &Client{
		BaseURL:  creds.URL,
		Username: creds.Username,
		Token:    creds.Token,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Crumb represents a Jenkins CSRF token
type Crumb struct {
	CrumbRequestField string `json:"crumbRequestField"`
	Crumb             string `json:"crumb"`
}

// doRequest performs an HTTP request with basic auth
func (c *Client) doRequest(method, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	url := c.BaseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	// Set basic auth
	req.SetBasicAuth(c.Username, c.Token)

	// Apply default headers
	if headers == nil || headers["Accept"] == "" {
		req.Header.Set("Accept", "application/json")
	}
	if body != nil && (headers == nil || headers["Content-Type"] == "") {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Attach crumb for mutating requests
	if requiresCrumb(method) {
		if err := c.ensureCrumb(); err != nil {
			return nil, err
		}
		if c.crumb != nil {
			req.Header.Set(c.crumb.CrumbRequestField, c.crumb.Crumb)
		}
	}

	return c.HTTPClient.Do(req)
}

func requiresCrumb(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		return true
	}
}

func (c *Client) ensureCrumb() error {
	c.crumbMu.Lock()
	defer c.crumbMu.Unlock()

	if c.crumb != nil || c.crumbDisabled {
		return nil
	}

	req, err := http.NewRequest(http.MethodGet, c.BaseURL+"/crumbIssuer/api/json", nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.Username, c.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to request crumb: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var crumb Crumb
		if err := json.NewDecoder(resp.Body).Decode(&crumb); err != nil {
			return fmt.Errorf("failed to decode crumb: %w", err)
		}
		if crumb.CrumbRequestField == "" || crumb.Crumb == "" {
			return fmt.Errorf("received empty crumb from Jenkins")
		}
		c.crumb = &crumb
		return nil

	case http.StatusNotFound, http.StatusForbidden:
		// Jenkins crumbs disabled or unsupported; continue without them.
		c.crumbDisabled = true
		return nil

	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to fetch crumb: status %d, body: %s", resp.StatusCode, string(body))
	}
}

// TestConnection tests the connection to Jenkins server
// Returns nil if successful, error otherwise
func (c *Client) TestConnection() error {
	resp, err := c.doRequest(http.MethodGet, "/api/json", nil, nil)
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
	resp, err := c.doRequest(http.MethodGet, "/api/json", nil, nil)
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

	resp, err := c.doRequest(http.MethodGet, path, nil, nil)
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

	resp, err := c.doRequest(http.MethodGet, path, nil, nil)
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

	resp, err := c.doRequest(http.MethodGet, path, nil, nil)
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

// GetJobDetails fetches detailed information about a specific job, including recent builds.
func (c *Client) GetJobDetails(fullName string, limit int) (*JobDetails, error) {
	if fullName == "" {
		return nil, fmt.Errorf("job name must not be empty")
	}

	if limit <= 0 || limit > 50 {
		limit = 10
	}

	jobPath := buildJobAPIPath(fullName)
	if jobPath == "" {
		return nil, fmt.Errorf("invalid job path for %q", fullName)
	}

	tree := fmt.Sprintf(
		"name,fullName,url,color,_class,description,"+
			"lastBuild[number,result,duration,timestamp,building,url,actions[causes[shortDescription,userId,userName],parameters[name,value],lastBuiltRevision[branch[SHA1,name]]]],"+
			"builds[number,result,duration,timestamp,building,url,actions[causes[shortDescription,userId,userName],parameters[name,value],lastBuiltRevision[branch[SHA1,name]]]]{%d},"+
			"property[parameterDefinitions[_class,name,type,description,trim,defaultValue,projectName,referencedParameters[name],defaultParameterValue[name,value],choices]]",
		limit,
	)

	params := url.Values{}
	params.Set("tree", tree)

	path := fmt.Sprintf("%s/api/json?%s", jobPath, params.Encode())

	resp, err := c.doRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch job details: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch job details: status %d, body: %s", resp.StatusCode, string(body))
	}

	var payload struct {
		JobDetails
		// Flatten property definitions to parameter list
		Property []struct {
			ParameterDefinitions []ParameterDefinition `json:"parameterDefinitions"`
		} `json:"property"`
	}

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode job details: %w", err)
	}

	details := payload.JobDetails
	if len(details.Builds) > limit {
		details.Builds = details.Builds[:limit]
	}
	for _, prop := range payload.Property {
		if len(prop.ParameterDefinitions) == 0 {
			continue
		}
		details.ParameterDefinitions = append(details.ParameterDefinitions, prop.ParameterDefinitions...)
	}

	return &details, nil
}

// TriggerBuild requests a new build for the specified job.
func (c *Client) TriggerBuild(fullName string) error {
	if fullName == "" {
		return fmt.Errorf("job name must not be empty")
	}

	jobPath := buildJobAPIPath(fullName)
	if jobPath == "" {
		return fmt.Errorf("invalid job path for %q", fullName)
	}

	path := fmt.Sprintf("%s/build?delay=0sec", jobPath)
	resp, err := c.doRequest(http.MethodPost, path, nil, map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	if err != nil {
		return fmt.Errorf("failed to trigger build: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		return nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to trigger build: status %d, body: %s", resp.StatusCode, string(body))
	}
}

// TriggerBuildWithParameters requests a new build providing parameter values.
func (c *Client) TriggerBuildWithParameters(fullName string, params map[string]string) error {
	if fullName == "" {
		return fmt.Errorf("job name must not be empty")
	}

	jobPath := buildJobAPIPath(fullName)
	if jobPath == "" {
		return fmt.Errorf("invalid job path for %q", fullName)
	}

	form := url.Values{}
	for key, value := range params {
		form.Set(key, value)
	}

	path := fmt.Sprintf("%s/buildWithParameters", jobPath)
	resp, err := c.doRequest(
		http.MethodPost,
		path,
		strings.NewReader(form.Encode()),
		map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
		},
	)
	if err != nil {
		return fmt.Errorf("failed to trigger build with parameters: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		return nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to trigger build with parameters: status %d, body: %s", resp.StatusCode, string(body))
	}
}

// AbortBuild sends a stop signal to a running build.
func (c *Client) AbortBuild(fullName string, buildNumber int) error {
	if fullName == "" {
		return fmt.Errorf("job name must not be empty")
	}
	if buildNumber <= 0 {
		return fmt.Errorf("build number must be greater than zero")
	}

	jobPath := buildJobAPIPath(fullName)
	if jobPath == "" {
		return fmt.Errorf("invalid job path for %q", fullName)
	}

	path := fmt.Sprintf("%s/%d/stop", jobPath, buildNumber)
	resp, err := c.doRequest(http.MethodPost, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to abort build: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusFound:
		return nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to abort build: status %d, body: %s", resp.StatusCode, string(body))
	}
}

// GetConsoleLog fetches the full console output for a specific build.
func (c *Client) GetConsoleLog(fullName string, buildNumber int) (string, error) {
	if fullName == "" {
		return "", fmt.Errorf("job name must not be empty")
	}
	if buildNumber <= 0 {
		return "", fmt.Errorf("build number must be greater than zero")
	}

	jobPath := buildJobAPIPath(fullName)
	if jobPath == "" {
		return "", fmt.Errorf("invalid job path for %q", fullName)
	}

	path := fmt.Sprintf("%s/%d/consoleText", jobPath, buildNumber)
	resp, err := c.doRequest(http.MethodGet, path, nil, map[string]string{
		"Accept": "text/plain",
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch console log: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to fetch console log: status %d, body: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read console log: %w", err)
	}
	return string(data), nil
}

// GetProgressiveLog fetches a chunk of console output using Jenkins' progressive log API.
// It returns the new content, the next offset to request, and whether more data is available.
// The lookup prefers the provided buildURL (if not empty) and falls back to job full name + build number.
func (c *Client) GetProgressiveLog(buildURL, fullName string, buildNumber int, start int64) (string, int64, bool, error) {
	if start < 0 {
		start = 0
	}

	logPath, err := c.progressiveLogPath(buildURL, fullName, buildNumber, start)
	if err != nil {
		return "", 0, false, err
	}

	resp, err := c.doRequest(http.MethodGet, logPath, nil, map[string]string{
		"Accept": "text/plain",
	})
	if err != nil {
		return "", 0, false, fmt.Errorf("failed to fetch progressive console log: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, false, fmt.Errorf("failed to fetch progressive console log: status %d, body: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, false, fmt.Errorf("failed to read progressive console log: %w", err)
	}

	nextOffset := start + int64(len(data))
	if sizeHeader := resp.Header.Get("X-Text-Size"); sizeHeader != "" {
		if parsed, parseErr := strconv.ParseInt(sizeHeader, 10, 64); parseErr == nil && parsed >= 0 {
			nextOffset = parsed
		}
	}

	more := false
	if strings.EqualFold(resp.Header.Get("X-More-Data"), "true") {
		more = true
	}

	return string(data), nextOffset, more, nil
}

func (c *Client) progressiveLogPath(buildURL, fullName string, buildNumber int, start int64) (string, error) {
	buildPath, err := c.resolveBuildPath(buildURL, fullName, buildNumber)
	if err != nil {
		return "", err
	}
	buildPath = strings.TrimSuffix(buildPath, "/")
	if buildPath == "" {
		return "", fmt.Errorf("resolved build path is empty")
	}
	return fmt.Sprintf("%s/logText/progressiveText?start=%d", buildPath, start), nil
}

func (c *Client) resolveBuildPath(buildURL, fullName string, buildNumber int) (string, error) {
	if trimmed := strings.TrimSpace(buildURL); trimmed != "" {
		if path, err := c.relativeBuildPath(trimmed); err == nil {
			return strings.TrimSuffix(path, "/"), nil
		}
	}

	if fullName == "" {
		return "", fmt.Errorf("job name must not be empty")
	}
	if buildNumber <= 0 {
		return "", fmt.Errorf("build number must be greater than zero")
	}

	jobPath := buildJobAPIPath(fullName)
	if jobPath == "" {
		return "", fmt.Errorf("invalid job path for %q", fullName)
	}

	return fmt.Sprintf("%s/%d", strings.TrimSuffix(jobPath, "/"), buildNumber), nil
}

func (c *Client) relativeBuildPath(buildURL string) (string, error) {
	trimmed := strings.TrimSpace(buildURL)
	if trimmed == "" {
		return "", fmt.Errorf("build URL must not be empty")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid build URL %q: %w", trimmed, err)
	}

	// If the URL was already relative, parsed.Scheme and parsed.Host will be empty.
	if parsed.Scheme == "" && parsed.Host == "" {
		path := parsed.Path
		if path == "" {
			return "", fmt.Errorf("build URL %q does not contain a path", trimmed)
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		return path, nil
	}

	baseURL, err := url.Parse(c.BaseURL)
	if err != nil {
		return "", fmt.Errorf("invalid Jenkins base URL %q: %w", c.BaseURL, err)
	}

	relPath := parsed.Path
	if relPath == "" {
		return "", fmt.Errorf("build URL %q does not contain a path", trimmed)
	}

	basePath := strings.TrimSuffix(baseURL.Path, "/")
	if basePath != "" && strings.HasPrefix(relPath, basePath) {
		relPath = strings.TrimPrefix(relPath, basePath)
	}

	if relPath == "" {
		return "", fmt.Errorf("build URL %q does not contain a path", trimmed)
	}

	if !strings.HasPrefix(relPath, "/") {
		relPath = "/" + relPath
	}

	return relPath, nil
}

// GetBuild fetches build details for the given job. When number <= 0 it returns the last (possibly running) build.
func (c *Client) GetBuild(fullName string, number int) (*Build, error) {
	if fullName == "" {
		return nil, fmt.Errorf("job name must not be empty")
	}

	jobPath := buildJobAPIPath(fullName)
	if jobPath == "" {
		return nil, fmt.Errorf("invalid job path for %q", fullName)
	}

	var path string
	if number <= 0 {
		path = fmt.Sprintf("%s/lastBuild/api/json", jobPath)
	} else {
		path = fmt.Sprintf("%s/%d/api/json", jobPath, number)
	}

	resp, err := c.doRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch build details: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch build details: status %d, body: %s", resp.StatusCode, string(body))
	}

	var build Build
	if err := json.NewDecoder(resp.Body).Decode(&build); err != nil {
		return nil, fmt.Errorf("failed to decode build details: %w", err)
	}

	return &build, nil
}

// GetJobConfig retrieves the raw job configuration (XML).
func (c *Client) GetJobConfig(fullName string) (string, error) {
	if fullName == "" {
		return "", fmt.Errorf("job name must not be empty")
	}

	jobPath := buildJobAPIPath(fullName)
	if jobPath == "" {
		return "", fmt.Errorf("invalid job path for %q", fullName)
	}

	path := fmt.Sprintf("%s/config.xml", jobPath)
	resp, err := c.doRequest(http.MethodGet, path, nil, map[string]string{
		"Accept": "application/xml",
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch job config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to fetch job config: status %d, body: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read job config: %w", err)
	}
	return string(data), nil
}

// buildJobAPIPath converts a Jenkins job full name (with / separators) into the /job/... API path.
func buildJobAPIPath(fullName string) string {
	if fullName == "" {
		return ""
	}

	segments := strings.Split(fullName, "/")
	var builder strings.Builder
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		builder.WriteString("/job/")
		builder.WriteString(url.PathEscape(segment))
	}

	return builder.String()
}

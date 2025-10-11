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

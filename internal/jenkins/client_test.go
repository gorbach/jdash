package jenkins

import (
	"testing"
)

func TestBuildJobAPIPath(t *testing.T) {
	tests := []struct {
		name     string
		fullName string
		want     string
	}{
		{
			name:     "simple job name",
			fullName: "my-job",
			want:     "/job/my-job",
		},
		{
			name:     "nested folder",
			fullName: "folder/my-job",
			want:     "/job/folder/job/my-job",
		},
		{
			name:     "deeply nested folders",
			fullName: "folder1/folder2/folder3/my-job",
			want:     "/job/folder1/job/folder2/job/folder3/job/my-job",
		},
		{
			name:     "job name with spaces",
			fullName: "my job",
			want:     "/job/my%20job",
		},
		{
			name:     "job name with special characters",
			fullName: "my-job@#$%",
			want:     "/job/my-job@%23$%25",
		},
		{
			name:     "URL encoding with slash in nested path",
			fullName: "folder/job with spaces",
			want:     "/job/folder/job/job%20with%20spaces",
		},
		{
			name:     "empty string",
			fullName: "",
			want:     "",
		},
		{
			name:     "single slash",
			fullName: "/",
			want:     "",
		},
		{
			name:     "multiple slashes",
			fullName: "///",
			want:     "",
		},
		{
			name:     "leading slash",
			fullName: "/my-job",
			want:     "/job/my-job",
		},
		{
			name:     "trailing slash",
			fullName: "my-job/",
			want:     "/job/my-job",
		},
		{
			name:     "empty segments in path",
			fullName: "folder//my-job",
			want:     "/job/folder/job/my-job",
		},
		{
			name:     "spaces only segment",
			fullName: "folder/   /my-job",
			want:     "/job/folder/job/my-job",
		},
		{
			name:     "mixed empty and spaces segments",
			fullName: "folder1//  //folder2/my-job",
			want:     "/job/folder1/job/folder2/job/my-job",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildJobAPIPath(tt.fullName)
			if got != tt.want {
				t.Errorf("buildJobAPIPath(%q) = %q, want %q", tt.fullName, got, tt.want)
			}
		})
	}
}

package jenkins

import (
	"testing"
)

func TestJob_GetStatus(t *testing.T) {
	tests := []struct {
		name string
		job  Job
		want string
	}{
		{
			name: "folder with jobs",
			job: Job{
				Name:  "my-folder",
				Jobs:  []Job{{Name: "child"}},
				Color: "notbuilt",
			},
			want: "FOLDER",
		},
		{
			name: "folder class - cloudbees",
			job: Job{
				Name:  "my-folder",
				Class: "com.cloudbees.hudson.plugins.folder.Folder",
				Color: "blue",
			},
			want: "FOLDER",
		},
		{
			name: "folder class - multibranch",
			job: Job{
				Name:  "my-multibranch",
				Class: "org.jenkinsci.plugins.workflow.multibranch.WorkflowMultiBranchProject",
				Color: "red",
			},
			want: "FOLDER",
		},
		{
			name: "never built - nil LastBuild",
			job: Job{
				Name:      "my-job",
				LastBuild: nil,
				Color:     "notbuilt",
			},
			want: "NEVER_BUILT",
		},
		{
			name: "building - LastBuild.Building is true",
			job: Job{
				Name: "my-job",
				LastBuild: &Build{
					Number:   1,
					Building: true,
				},
				Color: "blue_anime",
			},
			want: "BUILDING",
		},
		{
			name: "success - blue color",
			job: Job{
				Name: "my-job",
				LastBuild: &Build{
					Number:   1,
					Result:   "SUCCESS",
					Building: false,
				},
				Color: "blue",
			},
			want: "SUCCESS",
		},
		{
			name: "success - blue_anime color (building but not flagged)",
			job: Job{
				Name: "my-job",
				LastBuild: &Build{
					Number:   1,
					Result:   "SUCCESS",
					Building: false,
				},
				Color: "blue_anime",
			},
			want: "SUCCESS",
		},
		{
			name: "failed - red color",
			job: Job{
				Name: "my-job",
				LastBuild: &Build{
					Number:   1,
					Result:   "FAILURE",
					Building: false,
				},
				Color: "red",
			},
			want: "FAILED",
		},
		{
			name: "failed - red_anime color",
			job: Job{
				Name: "my-job",
				LastBuild: &Build{
					Number:   1,
					Result:   "FAILURE",
					Building: false,
				},
				Color: "red_anime",
			},
			want: "FAILED",
		},
		{
			name: "unstable - yellow color",
			job: Job{
				Name: "my-job",
				LastBuild: &Build{
					Number:   1,
					Result:   "UNSTABLE",
					Building: false,
				},
				Color: "yellow",
			},
			want: "UNSTABLE",
		},
		{
			name: "unstable - yellow_anime color",
			job: Job{
				Name: "my-job",
				LastBuild: &Build{
					Number:   1,
					Result:   "UNSTABLE",
					Building: false,
				},
				Color: "yellow_anime",
			},
			want: "UNSTABLE",
		},
		{
			name: "pending - grey color",
			job: Job{
				Name: "my-job",
				LastBuild: &Build{
					Number:   1,
					Building: false,
				},
				Color: "grey",
			},
			want: "PENDING",
		},
		{
			name: "disabled - disabled color",
			job: Job{
				Name: "my-job",
				LastBuild: &Build{
					Number:   1,
					Building: false,
				},
				Color: "disabled",
			},
			want: "DISABLED",
		},
		{
			name: "aborted - aborted color",
			job: Job{
				Name: "my-job",
				LastBuild: &Build{
					Number:   1,
					Result:   "ABORTED",
					Building: false,
				},
				Color: "aborted",
			},
			want: "ABORTED",
		},
		{
			name: "not built - notbuilt color",
			job: Job{
				Name: "my-job",
				LastBuild: &Build{
					Number:   1,
					Building: false,
				},
				Color: "notbuilt",
			},
			want: "NOT_BUILT",
		},
		{
			name: "unknown color with result",
			job: Job{
				Name: "my-job",
				LastBuild: &Build{
					Number:   1,
					Result:   "CUSTOM_RESULT",
					Building: false,
				},
				Color: "custom_color",
			},
			want: "CUSTOM_RESULT",
		},
		{
			name: "unknown color without result",
			job: Job{
				Name: "my-job",
				LastBuild: &Build{
					Number:   1,
					Building: false,
				},
				Color: "unknown_color",
			},
			want: "UNKNOWN",
		},
		{
			name: "empty color with LastBuild",
			job: Job{
				Name: "my-job",
				LastBuild: &Build{
					Number:   1,
					Building: false,
				},
				Color: "",
			},
			want: "UNKNOWN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.job.GetStatus()
			if got != tt.want {
				t.Errorf("Job.GetStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

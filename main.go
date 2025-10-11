package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorbach/jenkins-gotui/internal/app"
	"github.com/gorbach/jenkins-gotui/internal/auth"
)

func main() {
	// Check if we already have server config
	hasConfig := auth.HasServerConfig()

	if !hasConfig {
		// Show authentication screen
		authModel := auth.New()

		// Set callback to launch main app after successful auth
		var authenticated bool
		authModel.SetOnSuccess(func() {
			authenticated = true
		})

		p := tea.NewProgram(authModel, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// If not authenticated (user quit), exit
		if !authenticated {
			return
		}
	}

	// Load server config
	serverConfig, err := auth.GetServerConfig()
	if err != nil || serverConfig == nil {
		fmt.Fprintf(os.Stderr, "Failed to load server config: %v\n", err)
		os.Exit(1)
	}

	// Create Jenkins client
	client := auth.CreateJenkinsClient(serverConfig)

	// Launch main application
	appModel := app.New(serverConfig.URL, client)
	p := tea.NewProgram(appModel, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

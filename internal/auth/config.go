package auth

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/gorbach/jdash/internal/jenkins"
)

// ServerConfig holds Jenkins server credentials
type ServerConfig struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Token    string `json:"token"`
}

// UIConfig holds UI preferences
type UIConfig struct {
	RefreshInterval int    `json:"refreshInterval"`
	Theme           string `json:"theme"`
	CompactMode     bool   `json:"compactMode"`
}

// KeyBindings holds custom key bindings
type KeyBindings struct {
	Quit    string `json:"quit"`
	Refresh string `json:"refresh"`
	Search  string `json:"search"`
	Build   string `json:"build"`
}

// Config holds the complete application configuration
type Config struct {
	Server      *ServerConfig `json:"server"`
	UI          UIConfig      `json:"ui"`
	Keybindings KeyBindings   `json:"keybindings"`
}

var (
	configDir  string
	configFile string
)

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	configDir = filepath.Join(home, ".jdash")
	configFile = filepath.Join(configDir, "config.json")
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		Server: nil,
		UI: UIConfig{
			RefreshInterval: 5,
			Theme:           "dark",
			CompactMode:     false,
		},
		Keybindings: KeyBindings{
			Quit:    "q",
			Refresh: "r",
			Search:  "/",
			Build:   "b",
		},
	}
}

// ensureConfigDir creates the config directory if it doesn't exist
func ensureConfigDir() error {
	return os.MkdirAll(configDir, 0755)
}

// LoadConfig loads the configuration from disk or returns default config
func LoadConfig() (Config, error) {
	// Check if file exists
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return DefaultConfig(), err
	}

	// Parse config file
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return DefaultConfig(), err
	}

	// Merge with defaults to ensure all fields exist
	defaultCfg := DefaultConfig()
	if config.UI.RefreshInterval == 0 {
		config.UI = defaultCfg.UI
	}
	if config.Keybindings.Quit == "" {
		config.Keybindings = defaultCfg.Keybindings
	}

	return config, nil
}

// SaveConfig saves the configuration to disk
func SaveConfig(config Config) error {
	if err := ensureConfigDir(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configFile, data, 0644)
}

// SaveServerConfig saves only the server credentials
func SaveServerConfig(server ServerConfig) error {
	config, err := LoadConfig()
	if err != nil {
		config = DefaultConfig()
	}

	config.Server = &server
	return SaveConfig(config)
}

// HasServerConfig checks if server config exists
func HasServerConfig() bool {
	config, err := LoadConfig()
	if err != nil {
		return false
	}
	return config.Server != nil
}

// GetServerConfig retrieves the server config
func GetServerConfig() (*ServerConfig, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	return config.Server, nil
}

// CreateJenkinsClient creates a Jenkins client from server config
func CreateJenkinsClient(config *ServerConfig) *jenkins.Client {
	return jenkins.NewClient(jenkins.Credentials{
		URL:      config.URL,
		Username: config.Username,
		Token:    config.Token,
	})
}

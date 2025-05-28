package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// AppConfig stores application configuration
type AppConfig struct {
	APIEndpoint    string `json:"api_endpoint"`
	ClientID       string `json:"client_id"`
	ConnectionMode string `json:"connection_mode"` // auto, local, public
	LastConnected  string `json:"last_connected_to"`
	FirstRun       bool   `json:"first_run"`
}

const configFileName = "client_config.json"

var appConfig AppConfig

// LoadConfig loads configuration from file or creates default
func LoadConfig() AppConfig {
	configPath := getConfigFilePath()

	// Check if config file exists
	_, err := os.Stat(configPath)
	if os.IsNotExist(err) {
		// Create default config
		appConfig = AppConfig{
			APIEndpoint:    "ws://localhost:8000/ws/client/",
			ClientID:       fmt.Sprintf("client-%d", os.Getpid()),
			ConnectionMode: "auto",
			FirstRun:       true,
		}
		SaveConfig()
		return appConfig
	}

	// Read config file
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		fmt.Printf("⚠️ Error reading config file: %v, using defaults\n", err)
		return AppConfig{
			APIEndpoint:    "ws://localhost:8000/ws/client/",
			ClientID:       fmt.Sprintf("client-%d", os.Getpid()),
			ConnectionMode: "auto",
			FirstRun:       false,
		}
	}

	// Parse config
	err = json.Unmarshal(data, &appConfig)
	if err != nil {
		fmt.Printf("⚠️ Error parsing config file: %v, using defaults\n", err)
		return AppConfig{
			APIEndpoint:    "ws://localhost:8000/ws/client/",
			ClientID:       fmt.Sprintf("client-%d", os.Getpid()),
			ConnectionMode: "auto",
			FirstRun:       false,
		}
	}

	return appConfig
}

// SaveConfig saves current configuration to file
func SaveConfig() error {
	configPath := getConfigFilePath()

	// Ensure directory exists
	os.MkdirAll(filepath.Dir(configPath), 0755)

	// Serialize config
	data, err := json.MarshalIndent(appConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializing config: %v", err)
	}

	// Write to file
	if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("error writing config file: %v", err)
	}

	fmt.Println("✅ Configuration saved")
	return nil
}

// GetFullAPIEndpoint returns complete API endpoint with client ID
func GetFullAPIEndpoint() string {
	endpoint := appConfig.APIEndpoint
	if !strings.HasSuffix(endpoint, "/") {
		endpoint += "/"
	}
	return endpoint + appConfig.ClientID
}

// PromptForConfiguration asks user for configuration options
func PromptForConfiguration() {
	// Only prompt if it's the first run or local connection failed
	if !appConfig.FirstRun && !forceConfigPrompt {
		return
	}

	fmt.Println("\n⚙️ Configuration Setup")
	fmt.Println("------------------")

	// Ask about API endpoint
	var useCustomAPI string
	fmt.Print("🌐 Do you want to use a custom API relay server? (y/n): ")
	fmt.Scanln(&useCustomAPI)

	if strings.ToLower(useCustomAPI) == "y" || strings.ToLower(useCustomAPI) == "yes" {
		var customAPI string
		fmt.Print("🌐 Enter the API relay address (ex: ws://your-server.com:8000/ws/client/): ")
		fmt.Scanln(&customAPI)

		if customAPI != "" {
			appConfig.APIEndpoint = customAPI
			fmt.Println("✅ Custom API endpoint set")
		}
	} else {
		// Default endpoint
		appConfig.APIEndpoint = "ws://localhost:8000/ws/client/"
		fmt.Println("✅ Using default localhost API endpoint")
	}

	// Ask about custom client ID
	var useCustomID string
	fmt.Print("🆔 Do you want to use a custom client ID? (y/n): ")
	fmt.Scanln(&useCustomID)

	if strings.ToLower(useCustomID) == "y" || strings.ToLower(useCustomID) == "yes" {
		var clientID string
		fmt.Print("🆔 Enter your client ID: ")
		fmt.Scanln(&clientID)

		if clientID != "" {
			appConfig.ClientID = clientID
			fmt.Println("✅ Custom client ID set")
		}
	} else {
		// Generate one if not custom
		if appConfig.ClientID == "" {
			appConfig.ClientID = fmt.Sprintf("client-%d", os.Getpid())
		}
		fmt.Printf("✅ Using client ID: %s\n", appConfig.ClientID)
	}

	// Ask about connection mode
	var connectionPref string
	fmt.Print("🔄 Choose connection mode - [1]Auto (Default) [2]Local Only [3]Relay Only: ")
	fmt.Scanln(&connectionPref)

	switch connectionPref {
	case "2":
		appConfig.ConnectionMode = "local"
		fmt.Println("✅ Connection mode set to: Local Only")
	case "3":
		appConfig.ConnectionMode = "public"
		fmt.Println("✅ Connection mode set to: Relay Only")
	default:
		appConfig.ConnectionMode = "auto"
		fmt.Println("✅ Connection mode set to: Auto (try both)")
	}

	// No longer first run
	appConfig.FirstRun = false
	forceConfigPrompt = false

	// Save the changes
	SaveConfig()
}

// Helper functions
func getConfigFilePath() string {
	// On Windows: %APPDATA%\TLSClient\config.json
	// On Linux/macOS: ~/.tlsclient/config.json
	var configDir string

	if runtime.GOOS == "windows" {
		configDir = filepath.Join(os.Getenv("APPDATA"), "TLSClient")
	} else {
		configDir = filepath.Join(os.Getenv("HOME"), ".tlsclient")
	}

	return filepath.Join(configDir, configFileName)
}

// Global flag to force configuration prompt after connection failure
var forceConfigPrompt bool

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

// ServerConfig stores server configuration
type ServerConfig struct {
	APIEndpoint      string `json:"api_endpoint"`
	ServerID         string `json:"server_id"`
	ModulesDirectory string `json:"modules_directory"`
	DataDirectory    string `json:"data_directory"`
	FirstRun         bool   `json:"first_run"`
}

const configFileName = "server_config.json"

var serverConfig ServerConfig

// LoadServerConfig loads configuration from file or creates default
func LoadServerConfig() ServerConfig {
	configPath := getConfigFilePath()

	// Check if config file exists
	_, err := os.Stat(configPath)
	if os.IsNotExist(err) {
		// Create default config
		serverConfig = ServerConfig{
			APIEndpoint:      "ws://localhost:8000/ws/server/",
			ServerID:         fmt.Sprintf("main-server-%d", os.Getpid()),
			ModulesDirectory: "./modules",
			DataDirectory:    "./data",
			FirstRun:         true,
		}
		SaveServerConfig()
		return serverConfig
	}

	// Read config file
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		fmt.Printf("⚠️ Error reading config file: %v, using defaults\n", err)
		return ServerConfig{
			APIEndpoint:      "ws://localhost:8000/ws/server/",
			ServerID:         fmt.Sprintf("main-server-%d", os.Getpid()),
			ModulesDirectory: "./modules",
			DataDirectory:    "./data",
			FirstRun:         false,
		}
	}

	// Parse config
	err = json.Unmarshal(data, &serverConfig)
	if err != nil {
		fmt.Printf("⚠️ Error parsing config file: %v, using defaults\n", err)
		return ServerConfig{
			APIEndpoint:      "ws://localhost:8000/ws/server/",
			ServerID:         fmt.Sprintf("main-server-%d", os.Getpid()),
			ModulesDirectory: "./modules",
			DataDirectory:    "./data",
			FirstRun:         false,
		}
	}

	return serverConfig
}

// SaveServerConfig saves current configuration to file
func SaveServerConfig() error {
	configPath := getConfigFilePath()

	// Ensure directory exists
	os.MkdirAll(filepath.Dir(configPath), 0755)

	// Serialize config
	data, err := json.MarshalIndent(serverConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializing config: %v", err)
	}

	// Write to file
	if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("error writing config file: %v", err)
	}

	fmt.Println("✅ Server configuration saved")
	return nil
}

// GetFullAPIEndpoint returns complete API endpoint with server ID
func GetFullAPIEndpoint() string {
	endpoint := serverConfig.APIEndpoint
	if !strings.HasSuffix(endpoint, "/") {
		endpoint += "/"
	}
	return endpoint + serverConfig.ServerID
}

// PromptForServerConfiguration asks user for server configuration options
func PromptForServerConfiguration() {
	// Only prompt on first run or if forced
	if !serverConfig.FirstRun {
		// Check if explicitly requested via command line
		for _, arg := range os.Args {
			if arg == "--config" {
				break
			} else {
				return // Skip configuration if not first run and no explicit request
			}
		}
		return
	}

	fmt.Println("\n⚙️ Server Configuration Setup")
	fmt.Println("--------------------------")

	// Ask about API endpoint
	var useCustomAPI string
	fmt.Print("🌐 Do you want to use a custom API relay server? (y/n): ")
	fmt.Scanln(&useCustomAPI)

	if strings.ToLower(useCustomAPI) == "y" || strings.ToLower(useCustomAPI) == "yes" {
		var customAPI string
		fmt.Print("🌐 Enter the API relay address (ex: ws://your-server.com:8000/ws/server/): ")
		fmt.Scanln(&customAPI)

		if customAPI != "" {
			serverConfig.APIEndpoint = customAPI
			fmt.Println("✅ Custom API endpoint set")
		}
	} else {
		// Default endpoint
		serverConfig.APIEndpoint = "ws://localhost:8000/ws/server/"
		fmt.Println("✅ Using default localhost API endpoint")
	}

	// Ask about custom server ID
	var useCustomID string
	fmt.Print("🆔 Do you want to use a custom server ID? (y/n): ")
	fmt.Scanln(&useCustomID)

	if strings.ToLower(useCustomID) == "y" || strings.ToLower(useCustomID) == "yes" {
		var serverID string
		fmt.Print("🆔 Enter your server ID: ")
		fmt.Scanln(&serverID)

		if serverID != "" {
			serverConfig.ServerID = serverID
			fmt.Println("✅ Custom server ID set")
		}
	} else {
		// Generate if not already set
		if serverConfig.ServerID == "" {
			serverConfig.ServerID = fmt.Sprintf("main-server-%d", os.Getpid())
		}
		fmt.Printf("✅ Using server ID: %s\n", serverConfig.ServerID)
	}

	// Ask about custom directories
	var useCustomDirs string
	fmt.Print("📁 Do you want to customize directory locations? (y/n): ")
	fmt.Scanln(&useCustomDirs)

	if strings.ToLower(useCustomDirs) == "y" || strings.ToLower(useCustomDirs) == "yes" {
		var modulesDir, dataDir string

		fmt.Print("📁 Enter modules directory path (default: ./modules): ")
		fmt.Scanln(&modulesDir)
		if modulesDir != "" {
			serverConfig.ModulesDirectory = modulesDir
		}

		fmt.Print("📁 Enter data directory path (default: ./data): ")
		fmt.Scanln(&dataDir)
		if dataDir != "" {
			serverConfig.DataDirectory = dataDir
		}

		fmt.Println("✅ Directory paths updated")
	}

	// No longer first run
	serverConfig.FirstRun = false

	// Save the changes
	SaveServerConfig()
}

// Helper function to get config path
func getConfigFilePath() string {
	var configDir string

	if runtime.GOOS == "windows" {
		configDir = filepath.Join(os.Getenv("APPDATA"), "TLSServer")
	} else {
		configDir = filepath.Join(os.Getenv("HOME"), ".tlsserver")
	}

	return filepath.Join(configDir, configFileName)
}

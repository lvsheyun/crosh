package accelerator

import (
	"fmt"
	"runtime"

	"github.com/boomyao/crosh/internal/config"
	"github.com/boomyao/crosh/internal/mirror"
	"github.com/boomyao/crosh/internal/proxy"
)

// Manager orchestrates mirror and proxy acceleration
type Manager struct {
	config *config.Config
	xray   *proxy.XrayManager
}

// NewManager creates a new acceleration manager
func NewManager(cfg *config.Config) *Manager {
	xray := proxy.NewXrayManager(cfg.Proxy.XrayPath, cfg.Proxy.LocalPort)

	return &Manager{
		config: cfg,
		xray:   xray,
	}
}

// EnableMirrors enables all configured mirrors
func (m *Manager) EnableMirrors() error {
	if !m.config.Mirror.Enabled {
		return fmt.Errorf("mirrors are not enabled in config")
	}

	var errors []error

	// Enable NPM mirror
	if m.config.Mirror.NPM != "" {
		npm := mirror.NewNPMMirror(m.config.Mirror.NPM)
		if err := npm.Enable(); err != nil {
			errors = append(errors, fmt.Errorf("NPM mirror: %w", err))
		} else {
			fmt.Println("✓ NPM mirror enabled:", m.config.Mirror.NPM)
		}
	}

	// Enable Pip mirror
	if m.config.Mirror.Pip != "" {
		pip := mirror.NewPipMirror(m.config.Mirror.Pip)
		if err := pip.Enable(); err != nil {
			errors = append(errors, fmt.Errorf("Pip mirror: %w", err))
		} else {
			fmt.Println("✓ Pip mirror enabled:", m.config.Mirror.Pip)
		}
	}

	// Enable Apt mirror (Linux only)
	if m.config.Mirror.Apt != "" {
		apt := mirror.NewAptMirror(m.config.Mirror.Apt)
		if err := apt.Enable(); err != nil {
			// Don't fail on apt error (might not be Linux)
			fmt.Printf("⚠ Apt mirror skipped: %v\n", err)
		} else {
			fmt.Println("✓ Apt mirror enabled:", m.config.Mirror.Apt)
		}
	}

	// Enable Cargo mirror
	if m.config.Mirror.Cargo != "" {
		cargo := mirror.NewCargoMirror(m.config.Mirror.Cargo)
		if err := cargo.Enable(); err != nil {
			errors = append(errors, fmt.Errorf("Cargo mirror: %w", err))
		} else {
			fmt.Println("✓ Cargo mirror enabled:", m.config.Mirror.Cargo)
		}
	}

	// Enable Go proxy
	if m.config.Mirror.Go != "" {
		goMirror := mirror.NewGoMirror(m.config.Mirror.Go)
		if err := goMirror.Enable(); err != nil {
			errors = append(errors, fmt.Errorf("Go proxy: %w", err))
		} else {
			fmt.Println("✓ Go proxy enabled:", m.config.Mirror.Go)
		}
	}

	// Enable Docker registry mirrors
	dockerEnabled := false
	if len(m.config.Mirror.Docker) > 0 {
		dockerMirror := mirror.NewDockerMirror(m.config.Mirror.Docker)
		if err := dockerMirror.Enable(); err != nil {
			errors = append(errors, fmt.Errorf("Docker mirror: %w", err))
		} else {
			dockerEnabled = true
			// Format display string (remove https:// prefix for cleaner output)
			displayRegistries := make([]string, len(m.config.Mirror.Docker))
			for i, reg := range m.config.Mirror.Docker {
				displayRegistries[i] = reg
			}
			fmt.Printf("✓ Docker mirror enabled: %s\n", displayRegistries[0])
			if len(displayRegistries) > 1 {
				for _, reg := range displayRegistries[1:] {
					fmt.Printf("  Additional: %s\n", reg)
				}
			}
		}
	}

	if len(errors) > 0 {
		fmt.Printf("\n%d errors occurred:\n", len(errors))
		for _, err := range errors {
			fmt.Printf("  - %v\n", err)
		}
		return fmt.Errorf("some mirrors failed to enable")
	}

	// Show Docker restart instructions if Docker was enabled
	if dockerEnabled {
		m.printDockerRestartInstructions()
	}

	return nil
}

// DisableMirrors disables all mirrors
func (m *Manager) DisableMirrors() error {
	var errors []error

	// Disable NPM mirror
	npm := mirror.NewNPMMirror("")
	if err := npm.Disable(); err != nil {
		errors = append(errors, fmt.Errorf("NPM mirror: %w", err))
	} else {
		fmt.Println("✓ NPM mirror disabled")
	}

	// Disable Pip mirror
	pip := mirror.NewPipMirror("")
	if err := pip.Disable(); err != nil {
		errors = append(errors, fmt.Errorf("Pip mirror: %w", err))
	} else {
		fmt.Println("✓ Pip mirror disabled")
	}

	// Disable Apt mirror
	apt := mirror.NewAptMirror("")
	if err := apt.Disable(); err != nil {
		fmt.Printf("⚠ Apt mirror skipped: %v\n", err)
	} else {
		fmt.Println("✓ Apt mirror disabled")
	}

	// Disable Cargo mirror
	cargo := mirror.NewCargoMirror("")
	if err := cargo.Disable(); err != nil {
		errors = append(errors, fmt.Errorf("Cargo mirror: %w", err))
	} else {
		fmt.Println("✓ Cargo mirror disabled")
	}

	// Disable Go proxy
	goMirror := mirror.NewGoMirror("")
	if err := goMirror.Disable(); err != nil {
		errors = append(errors, fmt.Errorf("Go proxy: %w", err))
	} else {
		fmt.Println("✓ Go proxy disabled")
	}

	// Disable Docker registry mirrors
	dockerMirror := mirror.NewDockerMirror(nil)
	if err := dockerMirror.Disable(); err != nil {
		errors = append(errors, fmt.Errorf("Docker mirror: %w", err))
	} else {
		fmt.Println("✓ Docker mirror disabled")
	}

	if len(errors) > 0 {
		return fmt.Errorf("some mirrors failed to disable")
	}

	return nil
}

// GetMirrorStatus returns the status of all mirrors
func (m *Manager) GetMirrorStatus() map[string]string {
	status := make(map[string]string)

	// NPM status
	npm := mirror.NewNPMMirror(m.config.Mirror.NPM)
	if enabled, url, err := npm.Status(); err == nil {
		if enabled {
			status["NPM"] = url
		} else {
			status["NPM"] = "disabled"
		}
	}

	// Pip status
	pip := mirror.NewPipMirror(m.config.Mirror.Pip)
	if enabled, url, err := pip.Status(); err == nil {
		if enabled {
			status["Pip"] = url
		} else {
			status["Pip"] = "disabled"
		}
	}

	// Apt status
	apt := mirror.NewAptMirror(m.config.Mirror.Apt)
	if enabled, url, err := apt.Status(); err == nil {
		if enabled {
			status["Apt"] = url
		} else {
			status["Apt"] = "disabled"
		}
	}

	// Cargo status
	cargo := mirror.NewCargoMirror(m.config.Mirror.Cargo)
	if enabled, url, err := cargo.Status(); err == nil {
		if enabled {
			status["Cargo"] = url
		} else {
			status["Cargo"] = "disabled"
		}
	}

	// Go status
	goMirror := mirror.NewGoMirror(m.config.Mirror.Go)
	if enabled, url, err := goMirror.Status(); err == nil {
		if enabled {
			status["Go"] = url
		} else {
			status["Go"] = "disabled"
		}
	}

	// Docker status
	dockerMirror := mirror.NewDockerMirror(m.config.Mirror.Docker)
	if enabled, url, err := dockerMirror.Status(); err == nil {
		if enabled {
			status["Docker"] = url
		} else {
			// Use the custom message (e.g., "check Docker Desktop settings")
			status["Docker"] = url
		}
	}

	return status
}

// LoadProxyFromFile loads proxy configuration from a local YAML file
func (m *Manager) LoadProxyFromFile(filePath string) (*proxy.Subscription, error) {
	return proxy.LoadFromFile(filePath)
}

// EnableProxy enables proxy via Xray
func (m *Manager) EnableProxy() error {
	if !m.config.Proxy.Enabled {
		return fmt.Errorf("proxy is not enabled in config")
	}

	if m.config.Proxy.SubscriptionURL == "" {
		return fmt.Errorf("no subscription URL configured")
	}

	// Download Xray if needed
	if err := m.xray.Download(); err != nil {
		return fmt.Errorf("failed to download Xray: %w", err)
	}

	// Fetch subscription
	fmt.Println("Fetching subscription...")
	sub, err := proxy.FetchSubscription(m.config.Proxy.SubscriptionURL)
	if err != nil {
		return fmt.Errorf("failed to fetch subscription: %w", err)
	}

	fmt.Printf("Found %d nodes in subscription\n", len(sub.Nodes))

	// Select fastest node
	fmt.Println("Testing node latency...")
	node, err := sub.SelectFastestNode()
	if err != nil {
		return fmt.Errorf("failed to select node: %w", err)
	}

	fmt.Printf("Selected node: %s (latency: %dms)\n", node.Name, node.Latency)

	// Generate Xray config
	if err := m.xray.GenerateConfig(node); err != nil {
		return fmt.Errorf("failed to generate Xray config: %w", err)
	}

	// Start Xray
	if err := m.xray.Start(); err != nil {
		return fmt.Errorf("failed to start Xray: %w", err)
	}

	// Update config with current node
	m.config.Proxy.CurrentNode = node.Name
	if err := m.config.Save(); err != nil {
		fmt.Printf("Warning: failed to save config: %v\n", err)
	}

	// Print proxy environment variables
	fmt.Println("\nTo use the proxy, set these environment variables:")
	envVars := m.xray.GetProxyEnvVars()
	for key, value := range envVars {
		fmt.Printf("  export %s=%s\n", key, value)
	}

	return nil
}

// DisableProxy stops the proxy
func (m *Manager) DisableProxy() error {
	if err := m.xray.Stop(); err != nil {
		return err
	}

	m.config.Proxy.CurrentNode = ""
	m.config.Save()

	return nil
}

// GetProxyStatus returns the proxy status
func (m *Manager) GetProxyStatus() string {
	if m.xray.IsRunning() {
		return fmt.Sprintf("running (port %d, node: %s)", m.config.Proxy.LocalPort, m.config.Proxy.CurrentNode)
	}
	return "stopped"
}

// GetXrayManager returns the Xray manager instance
func (m *Manager) GetXrayManager() *proxy.XrayManager {
	return m.xray
}

// printDockerRestartInstructions prints instructions for restarting Docker daemon
func (m *Manager) printDockerRestartInstructions() {
	fmt.Println()
	fmt.Println("⚠ Docker daemon restart required to apply changes:")
	fmt.Println()

	// Detect OS and show appropriate restart instructions
	if runtime.GOOS == "darwin" {
		fmt.Println("  macOS (Docker Desktop):")
		fmt.Println("    killall Docker && open -a Docker")
	} else if runtime.GOOS == "linux" {
		fmt.Println("  Linux:")
		fmt.Println("    sudo systemctl restart docker")
	} else {
		// Windows or other
		fmt.Println("  Restart Docker Desktop from the system tray")
	}

	fmt.Println()
	fmt.Println("After restart, test with: docker pull nginx:alpine")
}

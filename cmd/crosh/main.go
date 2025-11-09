package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/boomyao/crosh/internal/accelerator"
	"github.com/boomyao/crosh/internal/config"
)

const version = "0.0.1"

func main() {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Create manager
	manager := accelerator.NewManager(cfg)

	// No arguments: default to "on"
	if len(os.Args) < 2 {
		handleOn(manager, cfg)
		return
	}

	arg := os.Args[1]

	// Check if argument is a URL (proxy subscription)
	if isHTTPURL(arg) {
		handleConfigureProxy(manager, cfg, arg)
		return
	}

	// Check if argument is a local YAML file
	if isYAMLFile(arg) {
		handleLocalYAMLFile(manager, cfg, arg)
		return
	}

	// Handle simple commands
	switch arg {
	case "on":
		handleOn(manager, cfg)
	case "off":
		handleOff(manager, cfg)
	case "status":
		handleStatus(manager, cfg)
	case "version", "-v", "--version":
		fmt.Printf("crosh version %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", arg)
		printUsage()
		os.Exit(1)
	}
}

// isHTTPURL checks if a string is an HTTP/HTTPS URL
func isHTTPURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// isYAMLFile checks if a string is a path to a YAML file
func isYAMLFile(s string) bool {
	if !strings.HasSuffix(s, ".yaml") && !strings.HasSuffix(s, ".yml") {
		return false
	}
	// Check if file exists
	if _, err := os.Stat(s); err == nil {
		return true
	}
	return false
}

func printUsage() {
	fmt.Println(`crosh - Network acceleration for Chinese developers

USAGE:
    crosh [command]

COMMANDS:
    (no args)           Enable acceleration (default)
    on                  Enable acceleration
    off                 Disable acceleration
    status              Show current status
    <subscription-url>  Configure proxy subscription and auto-start
    <config.yaml>       Use local YAML file (one-time configuration)
    version             Show version
    help                Show this help

EXAMPLES:
    # Enable acceleration
    crosh
    crosh on

    # Disable acceleration
    crosh off

    # Configure proxy subscription (auto-starts proxy and mirrors)
    crosh https://your-subscription-url

    # Use local YAML file (one-time use, not saved)
    crosh config.yaml
    crosh /path/to/proxies.yml

    # Check status
    crosh status

For more information, visit: https://github.com/boomyao/crosh`)
}

func handleOn(manager *accelerator.Manager, cfg *config.Config) {
	fmt.Println("Enabling acceleration...")
	fmt.Println()

	// Always enable mirrors (safe and beneficial)
	cfg.Mirror.Enabled = true
	if err := manager.EnableMirrors(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to enable mirrors: %v\n", err)
	} else {
		fmt.Println("✓ Mirrors enabled (npm, pip, apt, cargo, go)")
	}

	// Enable proxy if subscription is configured
	if cfg.Proxy.SubscriptionURL != "" {
		cfg.Proxy.Enabled = true
		if err := manager.EnableProxy(); err != nil {
			// If proxy fails, might be missing xray-core
			fmt.Fprintf(os.Stderr, "✗ Proxy failed: %v\n", err)
			fmt.Println("\nTrying to download Xray-core...")

			xray := manager.GetXrayManager()
			if downloadErr := xray.Download(); downloadErr != nil {
				fmt.Fprintf(os.Stderr, "✗ Failed to download Xray-core: %v\n", downloadErr)
				fmt.Println("\nProxy acceleration is unavailable.")
				fmt.Println("Mirrors are still enabled and working.")
			} else {
				// Retry enabling proxy after download
				if retryErr := manager.EnableProxy(); retryErr != nil {
					fmt.Fprintf(os.Stderr, "✗ Proxy still failed: %v\n", retryErr)
				} else {
					fmt.Println("✓ Proxy enabled")
				}
			}
		} else {
			fmt.Println("✓ Proxy enabled")
		}
	}

	cfg.Save()
	fmt.Println("\n✓ Acceleration enabled")
}

func handleOff(manager *accelerator.Manager, cfg *config.Config) {
	fmt.Println("Disabling acceleration...")
	fmt.Println()

	// Disable mirrors
	if err := manager.DisableMirrors(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to disable mirrors: %v\n", err)
	} else {
		fmt.Println("✓ Mirrors disabled")
	}

	// Disable proxy
	if err := manager.DisableProxy(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to disable proxy: %v\n", err)
	} else {
		if cfg.Proxy.Enabled {
			fmt.Println("✓ Proxy disabled")
		}
	}

	cfg.Mirror.Enabled = false
	cfg.Proxy.Enabled = false
	cfg.Save()

	fmt.Println("\n✓ Acceleration disabled")
}

func handleStatus(manager *accelerator.Manager, cfg *config.Config) {
	fmt.Println("Current Status")
	fmt.Println("==============")
	fmt.Println()

	// Mirror status
	if cfg.Mirror.Enabled {
		fmt.Println("✓ Mirrors: enabled")
		mirrorStatus := manager.GetMirrorStatus()
		for name, status := range mirrorStatus {
			if status != "disabled" {
				fmt.Printf("  • %s: %s\n", name, status)
			}
		}
	} else {
		fmt.Println("✗ Mirrors: disabled")
	}

	fmt.Println()

	// Proxy status
	if cfg.Proxy.SubscriptionURL != "" {
		if cfg.Proxy.Enabled {
			fmt.Printf("✓ Proxy: enabled (%s)\n", manager.GetProxyStatus())
		} else {
			fmt.Println("✗ Proxy: disabled")
		}
		fmt.Printf("  Subscription: %s\n", cfg.Proxy.SubscriptionURL)
	} else {
		fmt.Println("○ Proxy: not configured")
		fmt.Println("\n  To configure proxy, run:")
		fmt.Println("    crosh https://your-subscription-url")
	}
}

func handleConfigureProxy(manager *accelerator.Manager, cfg *config.Config, url string) {
	fmt.Printf("Configuring proxy subscription...\n\n")

	// Save subscription URL
	cfg.Proxy.SubscriptionURL = url
	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Subscription URL saved: %s\n", url)

	// Check if xray-core is installed
	if _, err := os.Stat(cfg.Proxy.XrayPath); os.IsNotExist(err) {
		fmt.Println("\nXray-core not found. Downloading...")
		xray := manager.GetXrayManager()
		if err := xray.Download(); err != nil {
			fmt.Fprintf(os.Stderr, "✗ Failed to download Xray-core: %v\n", err)
			fmt.Println("\nYou can try again later with: crosh on")
			return
		}
		fmt.Println("✓ Xray-core downloaded successfully")
	}

	fmt.Println("\n✓ Proxy configured successfully")

	// Automatically enable mirrors
	fmt.Println("\nEnabling mirrors...")
	cfg.Mirror.Enabled = true
	if err := manager.EnableMirrors(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to enable mirrors: %v\n", err)
	}

	// Automatically enable proxy
	fmt.Println("\nStarting proxy...")
	cfg.Proxy.Enabled = true
	if err := manager.EnableProxy(); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Failed to start proxy: %v\n", err)
		fmt.Println("\nYou can try again with: crosh on")
		return
	}

	cfg.Save()

	fmt.Println("\n✓ Acceleration enabled")
	fmt.Println("\nProxy is running in background.")
}

func handleLocalYAMLFile(manager *accelerator.Manager, cfg *config.Config, filePath string) {
	fmt.Printf("Loading proxy configuration from local YAML file...\n\n")

	// Clear subscription URL (one-time use, don't save file path)
	cfg.Proxy.SubscriptionURL = ""

	// Check if xray-core is installed
	if _, err := os.Stat(cfg.Proxy.XrayPath); os.IsNotExist(err) {
		fmt.Println("Xray-core not found. Downloading...")
		xray := manager.GetXrayManager()
		if err := xray.Download(); err != nil {
			fmt.Fprintf(os.Stderr, "✗ Failed to download Xray-core: %v\n", err)
			fmt.Println("\nPlease try again later.")
			return
		}
		fmt.Println("✓ Xray-core downloaded successfully")
	}

	// Load nodes from local YAML file
	fmt.Println("\nParsing YAML file...")
	sub, err := manager.LoadProxyFromFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Failed to load YAML file: %v\n", err)
		fmt.Println("\nPlease check your YAML file format and try again.")
		return
	}

	fmt.Printf("✓ Found %d nodes in YAML file\n", len(sub.Nodes))

	// Select fastest node
	fmt.Println("\nTesting node latency...")
	node, err := sub.SelectFastestNode()
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Failed to select node: %v\n", err)
		return
	}

	fmt.Printf("✓ Selected node: %s (latency: %dms)\n", node.Name, node.Latency)

	// Generate Xray config
	xray := manager.GetXrayManager()
	if err := xray.GenerateConfig(node); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Failed to generate Xray config: %v\n", err)
		return
	}

	fmt.Println("\n✓ Proxy configured successfully (one-time use)")

	// Automatically enable mirrors
	fmt.Println("\nEnabling mirrors...")
	cfg.Mirror.Enabled = true
	if err := manager.EnableMirrors(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to enable mirrors: %v\n", err)
	}

	// Start Xray
	fmt.Println("\nStarting proxy...")
	if err := xray.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Failed to start proxy: %v\n", err)
		return
	}

	cfg.Proxy.Enabled = true
	cfg.Proxy.CurrentNode = node.Name
	cfg.Save()

	// Print proxy environment variables
	fmt.Println("\n✓ Acceleration enabled")
	fmt.Println("\nProxy is running in background.")
	fmt.Println("\nTo use the proxy, set these environment variables:")
	envVars := xray.GetProxyEnvVars()
	for key, value := range envVars {
		fmt.Printf("  export %s=%s\n", key, value)
	}

	fmt.Println("\nNote: This is a one-time configuration. To use this YAML file again, run: crosh " + filePath)
}

package proxy

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// XraySource represents a download source with both API and download URLs
type XraySource struct {
	Name        string
	APIURL      string
	DownloadURL string
}

// Multiple download sources for Xray-core (for China network)
var xraySources = []XraySource{
	{
		Name:        "Cloudflare CDN (crosh mirror)",
		APIURL:      "https://crosh.boomyao.com/xray/VERSION",
		DownloadURL: "https://crosh.boomyao.com/xray",
	},
	{
		Name:        "Official GitHub",
		APIURL:      "https://api.github.com/repos/XTLS/Xray-core/releases/latest",
		DownloadURL: "https://github.com/XTLS/Xray-core/releases/download",
	},
}

// XrayManager manages Xray-core process
type XrayManager struct {
	xrayPath   string
	configPath string
	cmd        *exec.Cmd
	localPort  int
}

// NewXrayManager creates a new Xray manager
func NewXrayManager(xrayPath string, localPort int) *XrayManager {
	return &XrayManager{
		xrayPath:   xrayPath,
		configPath: filepath.Join(filepath.Dir(xrayPath), "config.json"),
		localPort:  localPort,
	}
}

// Download downloads Xray-core binary with multiple fallback sources
func (x *XrayManager) Download() error {
	// Check if already exists
	if _, err := os.Stat(x.xrayPath); err == nil {
		fmt.Println("Xray-core already exists, skipping download")
	} else {
		fmt.Println("Downloading Xray-core...")

		// Create directory
		if err := os.MkdirAll(filepath.Dir(x.xrayPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Get latest release info
		version, assetName, err := x.getLatestReleaseInfo()
		if err != nil {
			fmt.Printf("Warning: failed to get latest release info: %v\n", err)
			fmt.Println("Falling back to default version v1.8.4")
			version = "v1.8.4"
			assetName = x.getDefaultAssetName()
		}

		fmt.Printf("Downloading Xray-core version %s...\n", version)

		// Try multiple download sources
		var lastErr error
		for i, source := range xraySources {
			downloadURL := fmt.Sprintf("%s/%s/%s", source.DownloadURL, version, assetName)
			fmt.Printf("Trying source %d/%d: %s\n", i+1, len(xraySources), source.Name)

			err := x.downloadFromURL(downloadURL)
			if err == nil {
				fmt.Println("✓ Xray-core downloaded successfully")
				break
			}

			fmt.Printf("✗ Failed: %v\n", err)
			lastErr = err
		}

		if lastErr != nil {
			return fmt.Errorf("failed to download from all sources: %w", lastErr)
		}
	}

	// Download geoip and geosite data files
	fmt.Println("Downloading geoip and geosite data files...")
	if err := x.downloadGeoData(); err != nil {
		fmt.Printf("Warning: failed to download geo data: %v\n", err)
		fmt.Println("Routing rules may not work properly without geo data files")
	}

	return nil
}

// downloadGeoData downloads geoip.dat and geosite.dat files
func (x *XrayManager) downloadGeoData() error {
	dataDir := filepath.Dir(x.xrayPath)

	// Geo data file sources (Cloudflare CDN first for best China access)
	geoFiles := []struct {
		name     string
		sources  []string
		filename string
	}{
		{
			name:     "geoip.dat",
			filename: "geoip.dat",
			sources: []string{
				"https://crosh.boomyao.com/xray/geoip.dat",
				"https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat",
			},
		},
		{
			name:     "geosite.dat",
			filename: "geosite.dat",
			sources: []string{
				"https://crosh.boomyao.com/xray/geosite.dat",
				"https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat",
			},
		},
	}

	for _, geoFile := range geoFiles {
		targetPath := filepath.Join(dataDir, geoFile.filename)

		// Skip if file already exists
		if _, err := os.Stat(targetPath); err == nil {
			fmt.Printf("✓ %s already exists\n", geoFile.name)
			continue
		}

		fmt.Printf("Downloading %s...\n", geoFile.name)

		// Try multiple sources
		var lastErr error
		for i, source := range geoFile.sources {
			fmt.Printf("  Trying source %d/%d...\n", i+1, len(geoFile.sources))

			err := x.downloadGeoFile(source, targetPath)
			if err == nil {
				fmt.Printf("✓ %s downloaded successfully\n", geoFile.name)
				break
			}

			fmt.Printf("  ✗ Failed: %v\n", err)
			lastErr = err
		}

		if lastErr != nil {
			return fmt.Errorf("failed to download %s: %w", geoFile.name, lastErr)
		}
	}

	return nil
}

// downloadGeoFile downloads a single geo data file
func (x *XrayManager) downloadGeoFile(url, targetPath string) error {
	client := &http.Client{
		Timeout: 3 * time.Minute,
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Create temporary file
	tmpFile := targetPath + ".tmp"
	out, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	_, err = io.Copy(out, resp.Body)
	out.Close()

	if err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to save file: %w", err)
	}

	// Rename to final location
	if err := os.Rename(tmpFile, targetPath); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to move to final location: %w", err)
	}

	return nil
}

// downloadFromURL downloads Xray-core from a specific URL
func (x *XrayManager) downloadFromURL(downloadURL string) error {
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	resp, err := client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Save to temporary zip file
	tmpZip := x.xrayPath + ".tmp.zip"
	out, err := os.Create(tmpZip)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	_, err = io.Copy(out, resp.Body)
	out.Close()

	if err != nil {
		os.Remove(tmpZip)
		return fmt.Errorf("failed to save file: %w", err)
	}

	// Extract xray binary from zip
	if err := x.extractXrayFromZip(tmpZip); err != nil {
		os.Remove(tmpZip)
		return fmt.Errorf("failed to extract: %w", err)
	}

	// Clean up zip file
	os.Remove(tmpZip)

	return nil
}

// extractXrayFromZip extracts the xray binary from a zip file
func (x *XrayManager) extractXrayFromZip(zipPath string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	defer reader.Close()

	// Find the xray executable (could be named "xray" or "xray-core")
	var xrayFile *zip.File
	for _, file := range reader.File {
		name := filepath.Base(file.Name)
		if name == "xray" || name == "xray-core" {
			xrayFile = file
			break
		}
	}

	if xrayFile == nil {
		return fmt.Errorf("xray binary not found in zip")
	}

	// Extract the file
	src, err := xrayFile.Open()
	if err != nil {
		return fmt.Errorf("failed to open file in zip: %w", err)
	}
	defer src.Close()

	// Create destination file
	tmpFile := x.xrayPath + ".tmp"
	dst, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}

	_, err = io.Copy(dst, src)
	dst.Close()

	if err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Rename to final location
	if err := os.Rename(tmpFile, x.xrayPath); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to move to final location: %w", err)
	}

	return nil
}

// getDefaultAssetName returns the default asset name based on OS and architecture
func (x *XrayManager) getDefaultAssetName() string {
	osName, archName := getXrayPlatformNames()
	return fmt.Sprintf("Xray-%s-%s.zip", osName, archName)
}

// getXrayPlatformNames converts Go's GOOS/GOARCH to Xray-core naming convention
func getXrayPlatformNames() (osName, archName string) {
	// Map OS names
	switch runtime.GOOS {
	case "darwin":
		osName = "macos"
	case "windows":
		osName = "windows"
	case "linux":
		osName = "linux"
	case "freebsd":
		osName = "freebsd"
	case "openbsd":
		osName = "openbsd"
	default:
		osName = runtime.GOOS
	}

	// Map architecture names
	switch runtime.GOARCH {
	case "amd64":
		archName = "64"
	case "386":
		archName = "32"
	case "arm64":
		archName = "arm64-v8a"
	case "arm":
		archName = "arm32-v7a"
	case "mips64", "mips64le":
		archName = "mips64"
	case "mips", "mipsle":
		archName = "mips32"
	case "s390x":
		archName = "s390x"
	case "riscv64":
		archName = "riscv64"
	default:
		archName = runtime.GOARCH
	}

	return osName, archName
}

// getLatestReleaseInfo gets the latest release info from GitHub with proxy fallback
func (x *XrayManager) getLatestReleaseInfo() (version, assetName string, err error) {
	var lastErr error
	for _, source := range xraySources {
		// Special handling for Cloudflare CDN source
		if strings.Contains(source.Name, "Cloudflare") {
			version, assetName, err = x.getVersionFromCDN(source)
		} else {
			version, assetName, err = x.getVersionFromGitHub(source)
		}

		if err == nil {
			return version, assetName, nil
		}
		lastErr = err
	}

	return "", "", lastErr
}

// getVersionFromCDN fetches version info from Cloudflare CDN
func (x *XrayManager) getVersionFromCDN(source XraySource) (string, string, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(source.APIURL)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	versionBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	version := strings.TrimSpace(string(versionBytes))
	osName, archName := getXrayPlatformNames()
	assetName := fmt.Sprintf("Xray-%s-%s.zip", osName, archName)

	return version, assetName, nil
}

// getVersionFromGitHub fetches release info from GitHub API
func (x *XrayManager) getVersionFromGitHub(source XraySource) (version, assetName string, err error) {
	return x.fetchReleaseInfo(source.APIURL)
}

// fetchReleaseInfo fetches release info from a specific API endpoint
func (x *XrayManager) fetchReleaseInfo(apiURL string) (version, assetName string, err error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(apiURL)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", err
	}

	version = release.TagName

	// Determine the correct asset based on OS and architecture
	osName, archName := getXrayPlatformNames()
	assetPattern := fmt.Sprintf("Xray-%s-%s", osName, archName)

	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, assetPattern) && strings.HasSuffix(asset.Name, ".zip") {
			return version, asset.Name, nil
		}
	}

	return "", "", fmt.Errorf("no suitable binary found for %s/%s (looking for %s)", runtime.GOOS, runtime.GOARCH, assetPattern)
}

// GenerateConfig generates Xray configuration from a node
func (x *XrayManager) GenerateConfig(node *Node) error {
	var config map[string]interface{}

	switch node.Type {
	case "vmess":
		config = x.generateVMessConfig(node)
	case "vless":
		config = x.generateVLessConfig(node)
	case "trojan":
		config = x.generateTrojanConfig(node)
	case "ss":
		config = x.generateShadowsocksConfig(node)
	default:
		return fmt.Errorf("unsupported node type: %s", node.Type)
	}

	// Write config to file
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(x.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// generateRoutingRules generates routing rules for China IP direct connection
func (x *XrayManager) generateRoutingRules() map[string]interface{} {
	return map[string]interface{}{
		"domainStrategy": "IPIfNonMatch",
		"rules": []map[string]interface{}{
			{
				"type":        "field",
				"ip":          []string{"geoip:private"},
				"outboundTag": "direct",
			},
			{
				"type":        "field",
				"ip":          []string{"geoip:cn"},
				"outboundTag": "direct",
			},
			{
				"type":        "field",
				"domain":      []string{"geosite:cn"},
				"outboundTag": "direct",
			},
		},
	}
}

// generateDirectOutbound generates direct connection outbound
func (x *XrayManager) generateDirectOutbound() map[string]interface{} {
	return map[string]interface{}{
		"tag":      "direct",
		"protocol": "freedom",
		"settings": map[string]interface{}{},
	}
}

// generateVMessConfig generates VMess configuration
func (x *XrayManager) generateVMessConfig(node *Node) map[string]interface{} {
	proxyOutbound := map[string]interface{}{
		"tag":      "proxy",
		"protocol": "vmess",
		"settings": map[string]interface{}{
			"vnext": []map[string]interface{}{
				{
					"address": node.Server,
					"port":    node.Port,
					"users": []map[string]interface{}{
						{
							"id":       node.UUID,
							"alterId":  0,
							"security": "auto",
						},
					},
				},
			},
		},
	}

	return map[string]interface{}{
		"inbounds": []map[string]interface{}{
			{
				"port":     x.localPort,
				"protocol": "socks",
				"settings": map[string]interface{}{
					"udp": true,
				},
			},
		},
		"outbounds": []map[string]interface{}{
			proxyOutbound,
			x.generateDirectOutbound(),
		},
		"routing": x.generateRoutingRules(),
	}
}

// generateVLessConfig generates VLess configuration
func (x *XrayManager) generateVLessConfig(node *Node) map[string]interface{} {
	proxyOutbound := map[string]interface{}{
		"tag":      "proxy",
		"protocol": "vless",
		"settings": map[string]interface{}{
			"vnext": []map[string]interface{}{
				{
					"address": node.Server,
					"port":    node.Port,
					"users": []map[string]interface{}{
						{
							"id":         node.UUID,
							"encryption": "none",
						},
					},
				},
			},
		},
	}

	return map[string]interface{}{
		"inbounds": []map[string]interface{}{
			{
				"port":     x.localPort,
				"protocol": "socks",
				"settings": map[string]interface{}{
					"udp": true,
				},
			},
		},
		"outbounds": []map[string]interface{}{
			proxyOutbound,
			x.generateDirectOutbound(),
		},
		"routing": x.generateRoutingRules(),
	}
}

// generateTrojanConfig generates Trojan configuration
func (x *XrayManager) generateTrojanConfig(node *Node) map[string]interface{} {
	// Determine SNI - use explicit SNI if set, otherwise use server address
	sni := node.SNI
	if sni == "" {
		sni = node.Server
	}

	proxyOutbound := map[string]interface{}{
		"tag":      "proxy",
		"protocol": "trojan",
		"settings": map[string]interface{}{
			"servers": []map[string]interface{}{
				{
					"address":  node.Server,
					"port":     node.Port,
					"password": node.Password,
				},
			},
		},
		"streamSettings": map[string]interface{}{
			"network":  "tcp",
			"security": "tls",
			"tlsSettings": map[string]interface{}{
				"serverName":              sni,
				"allowInsecure":           false,
				"alpn":                    []string{"h2", "http/1.1"},
				"disableSystemRoot":       false,
				"enableSessionResumption": true,
			},
		},
	}

	return map[string]interface{}{
		"inbounds": []map[string]interface{}{
			{
				"port":     x.localPort,
				"protocol": "socks",
				"settings": map[string]interface{}{
					"udp": true,
				},
			},
		},
		"outbounds": []map[string]interface{}{
			proxyOutbound,
			x.generateDirectOutbound(),
		},
		"routing": x.generateRoutingRules(),
	}
}

// generateShadowsocksConfig generates Shadowsocks configuration
func (x *XrayManager) generateShadowsocksConfig(node *Node) map[string]interface{} {
	proxyOutbound := map[string]interface{}{
		"tag":      "proxy",
		"protocol": "shadowsocks",
		"settings": map[string]interface{}{
			"servers": []map[string]interface{}{
				{
					"address":  node.Server,
					"port":     node.Port,
					"method":   node.Security,
					"password": node.Password,
				},
			},
		},
	}

	return map[string]interface{}{
		"inbounds": []map[string]interface{}{
			{
				"port":     x.localPort,
				"protocol": "socks",
				"settings": map[string]interface{}{
					"udp": true,
				},
			},
		},
		"outbounds": []map[string]interface{}{
			proxyOutbound,
			x.generateDirectOutbound(),
		},
		"routing": x.generateRoutingRules(),
	}
}

// Start starts the Xray-core process
func (x *XrayManager) Start() error {
	// Check if Xray binary exists
	if _, err := os.Stat(x.xrayPath); os.IsNotExist(err) {
		return fmt.Errorf("xray-core not found, please run download first")
	}

	// Check if already running
	if x.IsRunning() {
		return fmt.Errorf("xray-core is already running")
	}

	// Create log file for background process
	logFile := filepath.Join(filepath.Dir(x.xrayPath), "xray.log")
	logFileHandle, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	// Start Xray process with output redirected to log file
	x.cmd = exec.Command(x.xrayPath, "run", "-config", x.configPath)
	x.cmd.Stdout = logFileHandle
	x.cmd.Stderr = logFileHandle

	if err := x.cmd.Start(); err != nil {
		logFileHandle.Close()
		return fmt.Errorf("failed to start Xray-core: %w", err)
	}

	// Close the file handle in the parent process (child process keeps its copy)
	logFileHandle.Close()

	fmt.Printf("Xray-core started on port %d (PID: %d)\n", x.localPort, x.cmd.Process.Pid)
	fmt.Printf("Logs: %s\n", logFile)

	// Save PID to file
	pidFile := filepath.Join(filepath.Dir(x.xrayPath), "xray.pid")
	os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", x.cmd.Process.Pid)), 0644)

	return nil
}

// Stop stops the Xray-core process
func (x *XrayManager) Stop() error {
	pidFile := filepath.Join(filepath.Dir(x.xrayPath), "xray.pid")

	// Try to stop via cmd object first
	if x.cmd != nil && x.cmd.Process != nil {
		if err := x.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to stop Xray-core: %w", err)
		}
		x.cmd.Wait()
		x.cmd = nil
	} else {
		// Try to stop via PID file (for processes started in previous sessions)
		data, err := os.ReadFile(pidFile)
		if err == nil {
			var pid int
			fmt.Sscanf(string(data), "%d", &pid)

			if pid > 0 {
				process, err := os.FindProcess(pid)
				if err == nil {
					// Try to kill the process
					if err := process.Kill(); err != nil {
						// Process might already be dead, that's ok
						fmt.Printf("Note: Process %d may have already stopped\n", pid)
					}
				}
			}
		}
	}

	// Remove PID file
	os.Remove(pidFile)

	fmt.Println("Xray-core stopped")
	return nil
}

// IsRunning checks if Xray-core is running
func (x *XrayManager) IsRunning() bool {
	if x.cmd != nil && x.cmd.Process != nil {
		// Check if process is still alive
		err := x.cmd.Process.Signal(os.Signal(nil))
		return err == nil
	}

	// Check PID file
	pidFile := filepath.Join(filepath.Dir(x.xrayPath), "xray.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}

	var pid int
	fmt.Sscanf(string(data), "%d", &pid)

	// Check if process with this PID exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(os.Signal(nil))
	return err == nil
}

// GetProxyEnvVars returns environment variables for using the proxy
func (x *XrayManager) GetProxyEnvVars() map[string]string {
	proxyURL := fmt.Sprintf("socks5://127.0.0.1:%d", x.localPort)
	return map[string]string{
		"HTTP_PROXY":  proxyURL,
		"HTTPS_PROXY": proxyURL,
		"ALL_PROXY":   proxyURL,
		"http_proxy":  proxyURL,
		"https_proxy": proxyURL,
		"all_proxy":   proxyURL,
	}
}

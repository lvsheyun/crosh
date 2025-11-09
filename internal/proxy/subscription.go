package proxy

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Node represents a proxy node
type Node struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // vmess, vless, trojan, ss, etc.
	Server   string `json:"server"`
	Port     int    `json:"port"`
	UUID     string `json:"uuid,omitempty"`
	Password string `json:"password,omitempty"`
	Network  string `json:"network,omitempty"`
	Security string `json:"security,omitempty"`
	TLS      string `json:"tls,omitempty"`
	SNI      string `json:"sni,omitempty"`
	Latency  int    `json:"latency,omitempty"` // in milliseconds
}

// Subscription represents a proxy subscription
type Subscription struct {
	URL   string
	Nodes []Node
}

// YAMLConfig represents the YAML subscription format
type YAMLConfig struct {
	Proxies []YAMLProxy `yaml:"proxies"`
}

// YAMLProxy represents a proxy node in YAML format
type YAMLProxy struct {
	Name           string `yaml:"name"`
	Server         string `yaml:"server"`
	Port           int    `yaml:"port"`
	Type           string `yaml:"type"`
	Password       string `yaml:"password,omitempty"`
	UUID           string `yaml:"uuid,omitempty"`
	Cipher         string `yaml:"cipher,omitempty"`
	SNI            string `yaml:"sni,omitempty"`
	Network        string `yaml:"network,omitempty"`
	SkipCertVerify bool   `yaml:"skip-cert-verify,omitempty"`
	UDP            bool   `yaml:"udp,omitempty"`
}

// LoadFromFile loads and parses a local YAML subscription file
func LoadFromFile(filePath string) (*Subscription, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	nodes, err := parseYAMLSubscription(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML file: %w", err)
	}

	return &Subscription{
		URL:   filePath, // Store file path for reference
		Nodes: nodes,
	}, nil
}

// FetchSubscription fetches and parses a subscription URL
func FetchSubscription(subscriptionURL string) (*Subscription, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(subscriptionURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch subscription: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("subscription returned status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read subscription data: %w", err)
	}

	// Try to decode base64
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		// Maybe it's not base64 encoded
		decoded = data
	}

	nodes, err := parseSubscription(string(decoded))
	if err != nil {
		return nil, err
	}

	return &Subscription{
		URL:   subscriptionURL,
		Nodes: nodes,
	}, nil
}

// parseSubscription parses subscription content
func parseSubscription(content string) ([]Node, error) {
	// Try to detect if content is YAML format
	// YAML format typically contains "proxies:" or starts with structured data
	if strings.Contains(content, "proxies:") || strings.Contains(content, "- {name:") {
		nodes, err := parseYAMLSubscription(content)
		if err == nil && len(nodes) > 0 {
			return nodes, nil
		}
		// If YAML parsing fails, fall through to try URL format
	}

	// Parse as URL format (original implementation)
	lines := strings.Split(content, "\n")
	nodes := []Node{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Try to parse as different formats
		if strings.HasPrefix(line, "vmess://") {
			node, err := parseVMessURL(line)
			if err == nil {
				nodes = append(nodes, node)
			}
		} else if strings.HasPrefix(line, "vless://") {
			node, err := parseVLessURL(line)
			if err == nil {
				nodes = append(nodes, node)
			}
		} else if strings.HasPrefix(line, "trojan://") {
			node, err := parseTrojanURL(line)
			if err == nil {
				nodes = append(nodes, node)
			}
		} else if strings.HasPrefix(line, "ss://") {
			node, err := parseShadowsocksURL(line)
			if err == nil {
				nodes = append(nodes, node)
			}
		}
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no valid nodes found in subscription")
	}

	return nodes, nil
}

// parseVMessURL parses a vmess:// URL
func parseVMessURL(vmessURL string) (Node, error) {
	// vmess://base64encoded
	encoded := strings.TrimPrefix(vmessURL, "vmess://")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return Node{}, fmt.Errorf("failed to decode vmess URL: %w", err)
	}

	var vmessConfig map[string]interface{}
	if err := json.Unmarshal(decoded, &vmessConfig); err != nil {
		return Node{}, fmt.Errorf("failed to parse vmess config: %w", err)
	}

	node := Node{
		Type: "vmess",
	}

	if v, ok := vmessConfig["ps"].(string); ok {
		node.Name = v
	}
	if v, ok := vmessConfig["add"].(string); ok {
		node.Server = v
	}
	if v, ok := vmessConfig["port"].(float64); ok {
		node.Port = int(v)
	}
	if v, ok := vmessConfig["id"].(string); ok {
		node.UUID = v
	}
	if v, ok := vmessConfig["net"].(string); ok {
		node.Network = v
	}
	if v, ok := vmessConfig["tls"].(string); ok {
		node.TLS = v
	}

	return node, nil
}

// parseVLessURL parses a vless:// URL
func parseVLessURL(vlessURL string) (Node, error) {
	// vless://uuid@server:port?params#name
	vlessURL = strings.TrimPrefix(vlessURL, "vless://")

	// Split by # to get name
	parts := strings.SplitN(vlessURL, "#", 2)
	name := ""
	if len(parts) == 2 {
		name, _ = url.QueryUnescape(parts[1])
		vlessURL = parts[0]
	}

	// Split by ? to get params
	parts = strings.SplitN(vlessURL, "?", 2)
	params := make(map[string]string)
	if len(parts) == 2 {
		query, _ := url.ParseQuery(parts[1])
		for k, v := range query {
			if len(v) > 0 {
				params[k] = v[0]
			}
		}
		vlessURL = parts[0]
	}

	// Parse uuid@server:port
	parts = strings.SplitN(vlessURL, "@", 2)
	if len(parts) != 2 {
		return Node{}, fmt.Errorf("invalid vless URL format")
	}

	uuid := parts[0]
	serverPort := strings.SplitN(parts[1], ":", 2)
	if len(serverPort) != 2 {
		return Node{}, fmt.Errorf("invalid vless server:port format")
	}

	port := 0
	fmt.Sscanf(serverPort[1], "%d", &port)

	node := Node{
		Type:   "vless",
		Name:   name,
		Server: serverPort[0],
		Port:   port,
		UUID:   uuid,
	}

	if v, ok := params["type"]; ok {
		node.Network = v
	}
	if v, ok := params["security"]; ok {
		node.Security = v
	}

	return node, nil
}

// parseTrojanURL parses a trojan:// URL
func parseTrojanURL(trojanURL string) (Node, error) {
	// trojan://password@server:port?params#name
	trojanURL = strings.TrimPrefix(trojanURL, "trojan://")

	// Split by # to get name
	parts := strings.SplitN(trojanURL, "#", 2)
	name := ""
	if len(parts) == 2 {
		name, _ = url.QueryUnescape(parts[1])
		trojanURL = parts[0]
	}

	// Split by ? to get params
	parts = strings.SplitN(trojanURL, "?", 2)
	if len(parts) == 2 {
		trojanURL = parts[0]
	}

	// Parse password@server:port
	parts = strings.SplitN(trojanURL, "@", 2)
	if len(parts) != 2 {
		return Node{}, fmt.Errorf("invalid trojan URL format")
	}

	password := parts[0]
	serverPort := strings.SplitN(parts[1], ":", 2)
	if len(serverPort) != 2 {
		return Node{}, fmt.Errorf("invalid trojan server:port format")
	}

	port := 0
	fmt.Sscanf(serverPort[1], "%d", &port)

	return Node{
		Type:     "trojan",
		Name:     name,
		Server:   serverPort[0],
		Port:     port,
		Password: password,
	}, nil
}

// parseShadowsocksURL parses a ss:// URL
func parseShadowsocksURL(ssURL string) (Node, error) {
	// ss://base64(method:password)@server:port#name
	ssURL = strings.TrimPrefix(ssURL, "ss://")

	// Split by # to get name
	parts := strings.SplitN(ssURL, "#", 2)
	name := ""
	if len(parts) == 2 {
		name, _ = url.QueryUnescape(parts[1])
		ssURL = parts[0]
	}

	// Split by @ to get method:password and server:port
	parts = strings.SplitN(ssURL, "@", 2)
	if len(parts) != 2 {
		return Node{}, fmt.Errorf("invalid shadowsocks URL format")
	}

	// Decode method:password
	decoded, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(parts[0])
		if err != nil {
			return Node{}, fmt.Errorf("failed to decode shadowsocks credentials: %w", err)
		}
	}

	credentials := strings.SplitN(string(decoded), ":", 2)
	if len(credentials) != 2 {
		return Node{}, fmt.Errorf("invalid shadowsocks credentials format")
	}

	method := credentials[0]
	password := credentials[1]

	// Parse server:port
	serverPort := strings.SplitN(parts[1], ":", 2)
	if len(serverPort) != 2 {
		return Node{}, fmt.Errorf("invalid shadowsocks server:port format")
	}

	port := 0
	fmt.Sscanf(serverPort[1], "%d", &port)

	return Node{
		Type:     "ss",
		Name:     name,
		Server:   serverPort[0],
		Port:     port,
		Password: password,
		Security: method,
	}, nil
}

// TestLatency tests the latency of a node
func (n *Node) TestLatency() error {
	start := time.Now()

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", n.Server, n.Port), 5*time.Second)
	if err != nil {
		n.Latency = -1 // Mark as unreachable
		return err
	}
	defer conn.Close()

	n.Latency = int(time.Since(start).Milliseconds())
	return nil
}

// SelectFastestNode selects the node with lowest latency
func (s *Subscription) SelectFastestNode() (*Node, error) {
	if len(s.Nodes) == 0 {
		return nil, fmt.Errorf("no nodes available")
	}

	var fastestNode *Node
	minLatency := int(^uint(0) >> 1) // Max int

	for i := range s.Nodes {
		if err := s.Nodes[i].TestLatency(); err != nil {
			continue
		}

		if s.Nodes[i].Latency >= 0 && s.Nodes[i].Latency < minLatency {
			minLatency = s.Nodes[i].Latency
			fastestNode = &s.Nodes[i]
		}
	}

	if fastestNode == nil {
		return nil, fmt.Errorf("no reachable nodes found")
	}

	return fastestNode, nil
}

// parseYAMLSubscription parses YAML format subscription
func parseYAMLSubscription(content string) ([]Node, error) {
	var config YAMLConfig
	if err := yaml.Unmarshal([]byte(content), &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if len(config.Proxies) == 0 {
		return nil, fmt.Errorf("no proxies found in YAML config")
	}

	nodes := make([]Node, 0, len(config.Proxies))
	for _, proxy := range config.Proxies {
		// Skip info nodes (like Traffic and Expire information)
		if proxy.Server == "" || proxy.Port == 0 {
			continue
		}

		node := Node{
			Name:   proxy.Name,
			Type:   proxy.Type,
			Server: proxy.Server,
			Port:   proxy.Port,
		}

		// Map fields based on proxy type
		switch proxy.Type {
		case "trojan":
			node.Password = proxy.Password
			node.SNI = proxy.SNI
			if proxy.SNI == "" {
				// Use server as SNI if not specified
				node.SNI = proxy.Server
			}
		case "vmess":
			node.UUID = proxy.UUID
			node.Network = proxy.Network
		case "vless":
			node.UUID = proxy.UUID
			node.Network = proxy.Network
		case "ss", "shadowsocks":
			node.Password = proxy.Password
			node.Security = proxy.Cipher
		}

		nodes = append(nodes, node)
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no valid proxy nodes found in YAML config")
	}

	return nodes, nil
}

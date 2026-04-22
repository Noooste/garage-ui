package config

import (
	"fmt"
	"net"
	"os"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// GarageTomlResult holds the values extracted from a garage.toml file.
type GarageTomlResult struct {
	Endpoint      string
	AdminEndpoint string
	AdminToken    string
	Region        string
}

// garageTomlFile represents the subset of garage.toml we care about.
type garageTomlFile struct {
	S3API struct {
		APIBindAddr string `toml:"api_bind_addr"`
		S3Region    string `toml:"s3_region"`
	} `toml:"s3_api"`
	Admin struct {
		APIBindAddr string `toml:"api_bind_addr"`
		AdminToken  string `toml:"admin_token"`
	} `toml:"admin"`
}

// ParseGarageToml reads a garage.toml file and extracts the values needed
// for garage-ui configuration.
func ParseGarageToml(path string) (*GarageTomlResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading garage.toml: %w", err)
	}

	var f garageTomlFile
	if err := toml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing garage.toml: %w", err)
	}

	if f.Admin.AdminToken == "" {
		return nil, fmt.Errorf("garage.toml: [admin].admin_token is required")
	}
	if f.Admin.APIBindAddr == "" {
		return nil, fmt.Errorf("garage.toml: [admin].api_bind_addr is required")
	}
	if f.S3API.APIBindAddr == "" {
		return nil, fmt.Errorf("garage.toml: [s3_api].api_bind_addr is required")
	}

	adminEndpoint, err := convertBindAddr(f.Admin.APIBindAddr)
	if err != nil {
		return nil, fmt.Errorf("garage.toml: converting admin api_bind_addr: %w", err)
	}

	s3Endpoint, err := convertBindAddr(f.S3API.APIBindAddr)
	if err != nil {
		return nil, fmt.Errorf("garage.toml: converting s3_api api_bind_addr: %w", err)
	}

	region := f.S3API.S3Region
	if region == "" {
		region = "garage"
	}

	return &GarageTomlResult{
		Endpoint:      s3Endpoint,
		AdminEndpoint: adminEndpoint,
		AdminToken:    f.Admin.AdminToken,
		Region:        region,
	}, nil
}

// convertBindAddr converts a bind address like "[::]:3900" into an HTTP
// endpoint like "http://127.0.0.1:3900". Wildcard hosts (::, 0.0.0.0, empty)
// are replaced with 127.0.0.1.
func convertBindAddr(bindAddr string) (string, error) {
	if bindAddr == "" {
		return "", fmt.Errorf("bind address is empty")
	}

	host, port, err := net.SplitHostPort(bindAddr)
	if err != nil {
		return "", fmt.Errorf("invalid bind address %q: %w", bindAddr, err)
	}

	if port == "" {
		return "", fmt.Errorf("bind address %q has no port", bindAddr)
	}

	switch host {
	case "", "::", "0.0.0.0":
		host = "127.0.0.1"
	}

	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}

	return fmt.Sprintf("http://%s:%s", host, port), nil
}

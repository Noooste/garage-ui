package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Server  ServerConfig  `mapstructure:"server"`
	Garage  GarageConfig  `mapstructure:"garage"`
	Auth    AuthConfig    `mapstructure:"auth"`
	CORS    CORSConfig    `mapstructure:"cors"`
	Logging LoggingConfig `mapstructure:"logging"`
}

// ServerConfig contains server-related configuration
type ServerConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Environment  string `mapstructure:"environment"`
	FrontendPath string `mapstructure:"frontend_path"` // Path to frontend dist directory
}

// GarageConfig contains Garage S3 connection settings
type GarageConfig struct {
	Endpoint       string `mapstructure:"endpoint"`
	Region         string `mapstructure:"region"`
	UseSSL         bool   `mapstructure:"use_ssl"`
	ForcePathStyle bool   `mapstructure:"force_path_style"`
	AdminEndpoint  string `mapstructure:"admin_endpoint"`
	AdminToken     string `mapstructure:"admin_token"`
}

// AuthConfig contains authentication configuration
type AuthConfig struct {
	Mode  string          `mapstructure:"mode"` // "none", "basic", or "oidc"
	Basic BasicAuthConfig `mapstructure:"basic"`
	OIDC  OIDCConfig      `mapstructure:"oidc"`
}

// BasicAuthConfig contains basic authentication settings
type BasicAuthConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// OIDCConfig contains OIDC authentication settings
type OIDCConfig struct {
	Enabled           bool     `mapstructure:"enabled"`
	ProviderName      string   `mapstructure:"provider_name"`
	ClientID          string   `mapstructure:"client_id"`
	ClientSecret      string   `mapstructure:"client_secret"`
	Scopes            []string `mapstructure:"scopes"`
	IssuerURL         string   `mapstructure:"issuer_url"`
	AuthURL           string   `mapstructure:"auth_url"`
	TokenURL          string   `mapstructure:"token_url"`
	UserinfoURL       string   `mapstructure:"userinfo_url"`
	SkipIssuerCheck   bool     `mapstructure:"skip_issuer_check"`
	SkipExpiryCheck   bool     `mapstructure:"skip_expiry_check"`
	EmailAttribute    string   `mapstructure:"email_attribute"`
	UsernameAttribute string   `mapstructure:"username_attribute"`
	NameAttribute     string   `mapstructure:"name_attribute"`
	RoleAttributePath string   `mapstructure:"role_attribute_path"`
	AdminRole         string   `mapstructure:"admin_role"`
	TLSSkipVerify     bool     `mapstructure:"tls_skip_verify"`
	SessionMaxAge     int      `mapstructure:"session_max_age"`
	CookieName        string   `mapstructure:"cookie_name"`
	CookieSecure      bool     `mapstructure:"cookie_secure"`
	CookieHTTPOnly    bool     `mapstructure:"cookie_http_only"`
	CookieSameSite    string   `mapstructure:"cookie_same_site"`
}

// CORSConfig contains CORS settings for frontend communication
type CORSConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowedMethods   []string `mapstructure:"allowed_methods"`
	AllowedHeaders   []string `mapstructure:"allowed_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// Load reads the configuration from the specified file
func Load(configPath string) (*Config, error) {
	// Set default config file name if not specified
	if configPath == "" {
		configPath = "config.yaml"
	}

	// Configure viper to read the config file
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// Allow environment variables to override config values
	// Environment variables take precedence over config file
	viper.AutomaticEnv()
	viper.SetEnvPrefix("GARAGE_UI")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Bind environment variables to config keys
	// This ensures env vars override config file values
	bindEnvVars()

	// Read the config file (optional - will use defaults and env vars if not found)
	if _, err := os.Stat(configPath); err == nil {
		if err := viper.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Unmarshal the config into the Config struct
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate the configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// bindEnvVars binds all environment variables to their corresponding config keys
func bindEnvVars() {
	// Server config
	viper.BindEnv("server.host", "GARAGE_UI_SERVER_HOST")
	viper.BindEnv("server.port", "GARAGE_UI_SERVER_PORT")
	viper.BindEnv("server.environment", "GARAGE_UI_SERVER_ENVIRONMENT")
	viper.BindEnv("server.frontend_path", "GARAGE_UI_SERVER_FRONTEND_PATH")

	// Garage config
	viper.BindEnv("garage.endpoint", "GARAGE_UI_GARAGE_ENDPOINT")
	viper.BindEnv("garage.region", "GARAGE_UI_GARAGE_REGION")
	viper.BindEnv("garage.use_ssl", "GARAGE_UI_GARAGE_USE_SSL")
	viper.BindEnv("garage.force_path_style", "GARAGE_UI_GARAGE_FORCE_PATH_STYLE")
	viper.BindEnv("garage.admin_endpoint", "GARAGE_UI_GARAGE_ADMIN_ENDPOINT")
	viper.BindEnv("garage.admin_token", "GARAGE_UI_GARAGE_ADMIN_TOKEN")

	// Auth config
	viper.BindEnv("auth.mode", "GARAGE_UI_AUTH_MODE")
	viper.BindEnv("auth.basic.username", "GARAGE_UI_AUTH_BASIC_USERNAME")
	viper.BindEnv("auth.basic.password", "GARAGE_UI_AUTH_BASIC_PASSWORD")

	// OIDC config
	viper.BindEnv("auth.oidc.enabled", "GARAGE_UI_AUTH_OIDC_ENABLED")
	viper.BindEnv("auth.oidc.provider_name", "GARAGE_UI_AUTH_OIDC_PROVIDER_NAME")
	viper.BindEnv("auth.oidc.client_id", "GARAGE_UI_AUTH_OIDC_CLIENT_ID")
	viper.BindEnv("auth.oidc.client_secret", "GARAGE_UI_AUTH_OIDC_CLIENT_SECRET")
	viper.BindEnv("auth.oidc.scopes", "GARAGE_UI_AUTH_OIDC_SCOPES")
	viper.BindEnv("auth.oidc.issuer_url", "GARAGE_UI_AUTH_OIDC_ISSUER_URL")
	viper.BindEnv("auth.oidc.auth_url", "GARAGE_UI_AUTH_OIDC_AUTH_URL")
	viper.BindEnv("auth.oidc.token_url", "GARAGE_UI_AUTH_OIDC_TOKEN_URL")
	viper.BindEnv("auth.oidc.userinfo_url", "GARAGE_UI_AUTH_OIDC_USERINFO_URL")
	viper.BindEnv("auth.oidc.skip_issuer_check", "GARAGE_UI_AUTH_OIDC_SKIP_ISSUER_CHECK")
	viper.BindEnv("auth.oidc.skip_expiry_check", "GARAGE_UI_AUTH_OIDC_SKIP_EXPIRY_CHECK")
	viper.BindEnv("auth.oidc.email_attribute", "GARAGE_UI_AUTH_OIDC_EMAIL_ATTRIBUTE")
	viper.BindEnv("auth.oidc.username_attribute", "GARAGE_UI_AUTH_OIDC_USERNAME_ATTRIBUTE")
	viper.BindEnv("auth.oidc.name_attribute", "GARAGE_UI_AUTH_OIDC_NAME_ATTRIBUTE")
	viper.BindEnv("auth.oidc.role_attribute_path", "GARAGE_UI_AUTH_OIDC_ROLE_ATTRIBUTE_PATH")
	viper.BindEnv("auth.oidc.admin_role", "GARAGE_UI_AUTH_OIDC_ADMIN_ROLE")
	viper.BindEnv("auth.oidc.tls_skip_verify", "GARAGE_UI_AUTH_OIDC_TLS_SKIP_VERIFY")
	viper.BindEnv("auth.oidc.session_max_age", "GARAGE_UI_AUTH_OIDC_SESSION_MAX_AGE")
	viper.BindEnv("auth.oidc.cookie_name", "GARAGE_UI_AUTH_OIDC_COOKIE_NAME")
	viper.BindEnv("auth.oidc.cookie_secure", "GARAGE_UI_AUTH_OIDC_COOKIE_SECURE")
	viper.BindEnv("auth.oidc.cookie_http_only", "GARAGE_UI_AUTH_OIDC_COOKIE_HTTP_ONLY")
	viper.BindEnv("auth.oidc.cookie_same_site", "GARAGE_UI_AUTH_OIDC_COOKIE_SAME_SITE")

	// CORS config
	viper.BindEnv("cors.enabled", "GARAGE_UI_CORS_ENABLED")
	viper.BindEnv("cors.allowed_origins", "GARAGE_UI_CORS_ALLOWED_ORIGINS")
	viper.BindEnv("cors.allowed_methods", "GARAGE_UI_CORS_ALLOWED_METHODS")
	viper.BindEnv("cors.allowed_headers", "GARAGE_UI_CORS_ALLOWED_HEADERS")
	viper.BindEnv("cors.allow_credentials", "GARAGE_UI_CORS_ALLOW_CREDENTIALS")
	viper.BindEnv("cors.max_age", "GARAGE_UI_CORS_MAX_AGE")

	// Logging config
	viper.BindEnv("logging.level", "GARAGE_UI_LOGGING_LEVEL")
	viper.BindEnv("logging.format", "GARAGE_UI_LOGGING_FORMAT")
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate server config
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	// Validate Garage config
	if c.Garage.Endpoint == "" {
		return fmt.Errorf("garage endpoint is required")
	}
	if c.Garage.AdminEndpoint == "" {
		return fmt.Errorf("garage admin_endpoint is required")
	}
	if c.Garage.AdminToken == "" {
		return fmt.Errorf("garage admin_token is required")
	}

	// Validate auth mode
	if c.Auth.Mode != "none" && c.Auth.Mode != "basic" && c.Auth.Mode != "oidc" {
		return fmt.Errorf("auth mode must be 'none', 'basic', or 'oidc', got: %s", c.Auth.Mode)
	}

	// Validate basic auth if enabled
	if c.Auth.Mode == "basic" {
		if c.Auth.Basic.Username == "" || c.Auth.Basic.Password == "" {
			return fmt.Errorf("basic auth username and password are required when auth mode is 'basic'")
		}
	}

	// Validate OIDC config if enabled
	if c.Auth.Mode == "oidc" {
		if c.Auth.OIDC.ClientID == "" {
			return fmt.Errorf("oidc client_id is required when auth mode is 'oidc'")
		}
		if c.Auth.OIDC.IssuerURL == "" {
			return fmt.Errorf("oidc issuer_url is required when auth mode is 'oidc'")
		}
		if len(c.Auth.OIDC.Scopes) == 0 {
			return fmt.Errorf("oidc scopes are required when auth mode is 'oidc'")
		}
	}

	return nil
}

// GetAddress returns the full server address (host:port)
func (c *Config) GetAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Server.Environment == "development"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.Server.Environment == "production"
}

package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/viper"

	"Noooste/garage-ui/pkg/logger"
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
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	Environment     string `mapstructure:"environment"`
	FrontendPath    string `mapstructure:"frontend_path"`     // Path to frontend dist directory
	Domain          string `mapstructure:"domain"`            // Domain name (e.g., garage-ui.example.com)
	Protocol        string `mapstructure:"protocol"`          // Protocol for internal communication (http/https)
	RootURL         string `mapstructure:"root_url"`          // Full external URL for redirects (e.g., https://garage-ui.example.com)
	MaxBodySize     int64  `mapstructure:"max_body_size"`     // Maximum request body size in bytes (default: 300MB)
	MaxHeaderSize   int    `mapstructure:"max_header_size"`   // Maximum request header size in bytes (default: 1MB)
	ReadBufferSize  int    `mapstructure:"read_buffer_size"`  // Read buffer size in bytes (default: 4KB)
	WriteBufferSize int    `mapstructure:"write_buffer_size"` // Write buffer size in bytes (default: 4KB)
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
	Admin      AdminAuthConfig `mapstructure:"admin"`
	OIDC       OIDCConfig      `mapstructure:"oidc"`
	Token      TokenAuthConfig `mapstructure:"token"`
	JWTPrivKey string          `mapstructure:"jwt_private_key"` // Ed25519 private key in PEM format for JWT signing (64 bytes)
}

// AdminAuthConfig contains admin authentication settings
type AdminAuthConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// TokenAuthConfig contains admin token authentication settings.
// When enabled, users can log in using the Garage admin token.
type TokenAuthConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// OIDCConfig contains OIDC authentication settings
type OIDCConfig struct {
	Enabled           bool     `mapstructure:"enabled"`
	ProviderName      string   `mapstructure:"provider_name"`
	ClientID          string   `mapstructure:"client_id"`
	ClientSecret      string   `mapstructure:"client_secret"`
	Scopes            []string `mapstructure:"scopes"`
	IssuerURL         string   `mapstructure:"issuer_url"`
	SkipIssuerCheck   bool     `mapstructure:"skip_issuer_check"`
	SkipExpiryCheck   bool     `mapstructure:"skip_expiry_check"`
	EmailAttribute    string   `mapstructure:"email_attribute"`
	UsernameAttribute string   `mapstructure:"username_attribute"`
	NameAttribute     string   `mapstructure:"name_attribute"`
	RoleAttributePath string   `mapstructure:"role_attribute_path"`
	AdminRole         string   `mapstructure:"admin_role"`
	AdminRoles        []string `mapstructure:"admin_roles"`
	TLSSkipVerify     bool     `mapstructure:"tls_skip_verify"`
	SessionMaxAge     int      `mapstructure:"session_max_age"`
	CookieName        string   `mapstructure:"cookie_name"`
	CookieSecure      bool     `mapstructure:"cookie_secure"`
	CookieHTTPOnly    bool     `mapstructure:"cookie_http_only"`
	CookieSameSite    string   `mapstructure:"cookie_same_site"`
}

// EffectiveAdminRoles returns the deduplicated list of admin roles drawn from
// both admin_role (legacy single-value) and admin_roles (list). A user is
// considered an admin if any of their roles matches any entry in this list.
func (o OIDCConfig) EffectiveAdminRoles() []string {
	seen := make(map[string]struct{}, len(o.AdminRoles)+1)
	var roles []string
	add := func(r string) {
		if r == "" {
			return
		}
		if _, ok := seen[r]; ok {
			return
		}
		seen[r] = struct{}{}
		roles = append(roles, r)
	}
	add(o.AdminRole)
	for _, r := range o.AdminRoles {
		add(r)
	}
	return roles
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

// LoadOption configures optional behaviour of Load.
type LoadOption func(*loadOptions)

type loadOptions struct {
	garageTomlPath string
}

// WithGarageToml tells Load to parse a garage.toml file and use its values as
// lowest-priority defaults (below YAML, below env vars).
func WithGarageToml(path string) LoadOption {
	return func(o *loadOptions) {
		o.garageTomlPath = path
	}
}

// Load reads the configuration from the specified file
func Load(configPath string, opts ...LoadOption) (*Config, error) {
	var lo loadOptions
	for _, fn := range opts {
		fn(&lo)
	}

	// Set default config file name if not specified
	if configPath == "" {
		configPath = "config.yaml"
	}

	// Configure viper to read the config file
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// Built-in defaults (lowest priority)
	viper.SetDefault("server.host", "::")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.environment", "production")
	viper.SetDefault("garage.force_path_style", true)
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "text")
	viper.SetDefault("auth.oidc.cookie_name", "garage_session")
	viper.SetDefault("auth.oidc.cookie_http_only", true)
	viper.SetDefault("auth.oidc.cookie_same_site", "lax")
	viper.SetDefault("auth.oidc.session_max_age", 86400)

	// If garage.toml path is provided, parse it and set values as viper
	// defaults. Defaults sit below config-file and env-var values in viper's
	// priority order, so YAML and env vars will still win.
	if lo.garageTomlPath != "" {
		tomlResult, err := ParseGarageToml(lo.garageTomlPath)
		if err != nil {
			return nil, fmt.Errorf("error parsing garage.toml: %w", err)
		}
		viper.SetDefault("garage.endpoint", tomlResult.Endpoint)
		viper.SetDefault("garage.admin_endpoint", tomlResult.AdminEndpoint)
		viper.SetDefault("garage.admin_token", tomlResult.AdminToken)
		viper.SetDefault("garage.region", tomlResult.Region)
	}

	// Allow environment variables to override config values
	// Environment variables take precedence over config file
	viper.AutomaticEnv()
	viper.SetEnvPrefix("GARAGE_UI")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Env vars override config file values
	bindEnvVars()

	// Resolve `_FILE`-suffixed env vars for sensitive values (e.g.
	// {ENV}_FILE=/run/secrets/foo). Must run after bindEnvVars so the
	// warning about both forms being set fires correctly.
	if err := applyFileBackedEnvVars(); err != nil {
		return nil, fmt.Errorf("error resolving _FILE env vars: %w", err)
	}

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
	viper.BindEnv("server.domain", "GARAGE_UI_SERVER_DOMAIN")
	viper.BindEnv("server.protocol", "GARAGE_UI_SERVER_PROTOCOL")
	viper.BindEnv("server.root_url", "GARAGE_UI_SERVER_ROOT_URL")
	viper.BindEnv("server.max_body_size", "GARAGE_UI_SERVER_MAX_BODY_SIZE")
	viper.BindEnv("server.max_header_size", "GARAGE_UI_SERVER_MAX_HEADER_SIZE")
	viper.BindEnv("server.read_buffer_size", "GARAGE_UI_SERVER_READ_BUFFER_SIZE")
	viper.BindEnv("server.write_buffer_size", "GARAGE_UI_SERVER_WRITE_BUFFER_SIZE")

	// Garage config
	viper.BindEnv("garage.endpoint", "GARAGE_UI_GARAGE_ENDPOINT")
	viper.BindEnv("garage.region", "GARAGE_UI_GARAGE_REGION")
	viper.BindEnv("garage.use_ssl", "GARAGE_UI_GARAGE_USE_SSL")
	viper.BindEnv("garage.force_path_style", "GARAGE_UI_GARAGE_FORCE_PATH_STYLE")
	viper.BindEnv("garage.admin_endpoint", "GARAGE_UI_GARAGE_ADMIN_ENDPOINT")
	viper.BindEnv("garage.admin_token", "GARAGE_UI_GARAGE_ADMIN_TOKEN")

	// Auth config
	viper.BindEnv("auth.admin.enabled", "GARAGE_UI_AUTH_ADMIN_ENABLED")
	viper.BindEnv("auth.admin.username", "GARAGE_UI_AUTH_ADMIN_USERNAME")
	viper.BindEnv("auth.admin.password", "GARAGE_UI_AUTH_ADMIN_PASSWORD")
	viper.BindEnv("auth.jwt_private_key", "GARAGE_UI_AUTH_JWT_PRIVATE_KEY")

	// Token auth config
	viper.BindEnv("auth.token.enabled", "GARAGE_UI_AUTH_TOKEN_ENABLED")

	// OIDC config
	viper.BindEnv("auth.oidc.enabled", "GARAGE_UI_AUTH_OIDC_ENABLED")
	viper.BindEnv("auth.oidc.provider_name", "GARAGE_UI_AUTH_OIDC_PROVIDER_NAME")
	viper.BindEnv("auth.oidc.client_id", "GARAGE_UI_AUTH_OIDC_CLIENT_ID")
	viper.BindEnv("auth.oidc.client_secret", "GARAGE_UI_AUTH_OIDC_CLIENT_SECRET")
	viper.BindEnv("auth.oidc.scopes", "GARAGE_UI_AUTH_OIDC_SCOPES")
	viper.BindEnv("auth.oidc.issuer_url", "GARAGE_UI_AUTH_OIDC_ISSUER_URL")
	viper.BindEnv("auth.oidc.skip_issuer_check", "GARAGE_UI_AUTH_OIDC_SKIP_ISSUER_CHECK")
	viper.BindEnv("auth.oidc.skip_expiry_check", "GARAGE_UI_AUTH_OIDC_SKIP_EXPIRY_CHECK")
	viper.BindEnv("auth.oidc.email_attribute", "GARAGE_UI_AUTH_OIDC_EMAIL_ATTRIBUTE")
	viper.BindEnv("auth.oidc.username_attribute", "GARAGE_UI_AUTH_OIDC_USERNAME_ATTRIBUTE")
	viper.BindEnv("auth.oidc.name_attribute", "GARAGE_UI_AUTH_OIDC_NAME_ATTRIBUTE")
	viper.BindEnv("auth.oidc.role_attribute_path", "GARAGE_UI_AUTH_OIDC_ROLE_ATTRIBUTE_PATH")
	viper.BindEnv("auth.oidc.admin_role", "GARAGE_UI_AUTH_OIDC_ADMIN_ROLE")
	viper.BindEnv("auth.oidc.admin_roles", "GARAGE_UI_AUTH_OIDC_ADMIN_ROLES")
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

// fileBackedEnvVars maps env var names to viper config keys for variables that
// support the `_FILE` suffix convention. Operators may set `{ENV}_FILE` to a
// file path; the file's contents (with trailing whitespace trimmed) become the
// effective value. This pattern is used by Docker Official Images (postgres,
// mysql) to inject secrets via mounted files instead of plain env vars,
// avoiding exposure through `docker inspect`, process listings, or crash logs.
//
// Scope is intentionally limited to values that an operator would reasonably
// store in a Kubernetes Secret or Docker secret. Non-sensitive config (host,
// port, endpoints, etc.) is excluded.
var fileBackedEnvVars = map[string]string{
	"GARAGE_UI_GARAGE_ADMIN_TOKEN":      "garage.admin_token",
	"GARAGE_UI_AUTH_ADMIN_USERNAME":     "auth.admin.username",
	"GARAGE_UI_AUTH_ADMIN_PASSWORD":     "auth.admin.password",
	"GARAGE_UI_AUTH_JWT_PRIVATE_KEY":    "auth.jwt_private_key",
	"GARAGE_UI_AUTH_OIDC_CLIENT_ID":     "auth.oidc.client_id",
	"GARAGE_UI_AUTH_OIDC_CLIENT_SECRET": "auth.oidc.client_secret",
}

// applyFileBackedEnvVars resolves `_FILE`-suffixed env vars listed in
// fileBackedEnvVars. For each entry where `{ENV}_FILE` is set, the file is
// read and its contents (trimmed of trailing CR/LF) become the value via
// viper.Set, which is the highest-priority source — so a `_FILE` value wins
// over both `{ENV}` and YAML. A missing or unreadable file is a hard error.
func applyFileBackedEnvVars() error {
	for envVar, configKey := range fileBackedEnvVars {
		path := os.Getenv(envVar + "_FILE")
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s_FILE (%s): %w", envVar, path, err)
		}
		if os.Getenv(envVar) != "" {
			logger.Warn().
				Str("env", envVar).
				Msg("both VAR and VAR_FILE are set; VAR_FILE takes precedence")
		}
		viper.Set(configKey, strings.TrimRight(string(data), "\r\n"))
	}
	return nil
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

	// Validate admin auth if enabled
	if c.Auth.Admin.Enabled {
		if c.Auth.Admin.Username == "" || c.Auth.Admin.Password == "" {
			return fmt.Errorf("admin auth username and password are required when admin auth is enabled")
		}
	}

	// Validate OIDC config if enabled
	if c.Auth.OIDC.Enabled {
		if c.Auth.OIDC.ClientID == "" {
			return fmt.Errorf("oidc client_id is required when oidc is enabled")
		}
		if c.Auth.OIDC.IssuerURL == "" {
			return fmt.Errorf("oidc issuer_url is required when oidc is enabled")
		}
		if c.Server.RootURL == "" {
			return fmt.Errorf("server.root_url is required when oidc is enabled")
		}
		if len(c.Auth.OIDC.Scopes) == 0 {
			return fmt.Errorf("oidc scopes are required when oidc is enabled")
		}
		// Every authenticated route on this service grants full admin
		// access — there is no separate authorization layer. Empty
		// admin role configuration would therefore promote every user
		// in the IdP realm to cluster admin. Require operators to opt
		// in explicitly via admin_role or admin_roles.
		if len(c.Auth.OIDC.EffectiveAdminRoles()) == 0 {
			return fmt.Errorf("oidc admin_role or admin_roles is required when oidc is enabled: leaving them empty would grant cluster-admin access to any authenticated IdP user")
		}
	}

	return nil
}

// GetAddress returns the full server address (host:port)
func (c *Config) GetAddress() string {
	return net.JoinHostPort(c.Server.Host, strconv.Itoa(c.Server.Port))
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Server.Environment == "development"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.Server.Environment == "production"
}

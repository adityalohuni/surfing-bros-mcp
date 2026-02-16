package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/google/uuid"
)

const (
	defaultDaemonAddr      = ":9099"
	defaultClientMaxIdle   = 30 * time.Minute
	defaultRefreshInterval = 2 * time.Second
	defaultConfigDirName   = "surfingbros"
	defaultConfigFileName  = "config.toml"
)

type Settings struct {
	Path               string
	DaemonAddr         string
	MCPToken           string
	AdminToken         string
	ClientMaxIdle      time.Duration
	AdminBaseURL       string
	TUIRefreshInterval time.Duration
}

type fileConfig struct {
	Daemon daemonConfig `toml:"daemon"`
	Auth   authConfig   `toml:"auth"`
	TUI    tuiConfig    `toml:"tui"`
}

type daemonConfig struct {
	Addr          string `toml:"addr"`
	ClientMaxIdle string `toml:"client_max_idle"`
}

type authConfig struct {
	MCPToken   string `toml:"mcp_token"`
	AdminToken string `toml:"admin_token"`
}

type tuiConfig struct {
	AdminBaseURL    string `toml:"admin_base_url"`
	RefreshInterval string `toml:"refresh_interval"`
}

func LoadOrCreate(path string) (Settings, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return Settings{}, err
		}
	}

	cfg := defaultFileConfig()
	exists := false
	if _, err := os.Stat(path); err == nil {
		exists = true
		var onDisk fileConfig
		if _, err := toml.DecodeFile(path, &onDisk); err != nil {
			return Settings{}, fmt.Errorf("decode config %s: %w", path, err)
		}
		mergeFileConfig(&cfg, onDisk)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Settings{}, fmt.Errorf("stat config %s: %w", path, err)
	}

	changed := false
	if strings.TrimSpace(cfg.Auth.MCPToken) == "" {
		cfg.Auth.MCPToken = randomToken()
		changed = true
	}
	if strings.TrimSpace(cfg.Auth.AdminToken) == "" {
		cfg.Auth.AdminToken = randomToken()
		changed = true
	}
	if strings.TrimSpace(cfg.TUI.AdminBaseURL) == "" {
		cfg.TUI.AdminBaseURL = deriveAdminBaseURL(cfg.Daemon.Addr)
		changed = true
	}
	if strings.TrimSpace(cfg.TUI.RefreshInterval) == "" {
		cfg.TUI.RefreshInterval = defaultRefreshInterval.String()
		changed = true
	}
	if strings.TrimSpace(cfg.Daemon.ClientMaxIdle) == "" {
		cfg.Daemon.ClientMaxIdle = defaultClientMaxIdle.String()
		changed = true
	}
	if strings.TrimSpace(cfg.Daemon.Addr) == "" {
		cfg.Daemon.Addr = defaultDaemonAddr
		changed = true
	}

	if !exists || changed {
		if err := writeConfig(path, cfg); err != nil {
			return Settings{}, err
		}
	}

	settings, err := toSettings(path, cfg)
	if err != nil {
		return Settings{}, err
	}
	return settings, nil
}

// Save writes settings to disk and returns the normalized values loaded back
// from the config file (including defaults and generated tokens when needed).
func Save(settings Settings) (Settings, error) {
	path := strings.TrimSpace(settings.Path)
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return Settings{}, err
		}
	}

	cfg := fileConfig{
		Daemon: daemonConfig{
			Addr:          settings.DaemonAddr,
			ClientMaxIdle: settings.ClientMaxIdle.String(),
		},
		Auth: authConfig{
			MCPToken:   settings.MCPToken,
			AdminToken: settings.AdminToken,
		},
		TUI: tuiConfig{
			AdminBaseURL:    settings.AdminBaseURL,
			RefreshInterval: settings.TUIRefreshInterval.String(),
		},
	}

	if strings.TrimSpace(cfg.Daemon.ClientMaxIdle) == "" {
		cfg.Daemon.ClientMaxIdle = defaultClientMaxIdle.String()
	}
	if strings.TrimSpace(cfg.TUI.RefreshInterval) == "" {
		cfg.TUI.RefreshInterval = defaultRefreshInterval.String()
	}

	if err := writeConfig(path, cfg); err != nil {
		return Settings{}, err
	}
	return LoadOrCreate(path)
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", defaultConfigDirName, defaultConfigFileName), nil
}

func defaultFileConfig() fileConfig {
	return fileConfig{
		Daemon: daemonConfig{
			Addr:          defaultDaemonAddr,
			ClientMaxIdle: defaultClientMaxIdle.String(),
		},
		TUI: tuiConfig{
			RefreshInterval: defaultRefreshInterval.String(),
		},
	}
}

func mergeFileConfig(dst *fileConfig, src fileConfig) {
	if v := strings.TrimSpace(src.Daemon.Addr); v != "" {
		dst.Daemon.Addr = v
	}
	if v := strings.TrimSpace(src.Daemon.ClientMaxIdle); v != "" {
		dst.Daemon.ClientMaxIdle = v
	}
	if v := strings.TrimSpace(src.Auth.MCPToken); v != "" {
		dst.Auth.MCPToken = v
	}
	if v := strings.TrimSpace(src.Auth.AdminToken); v != "" {
		dst.Auth.AdminToken = v
	}
	if v := strings.TrimSpace(src.TUI.AdminBaseURL); v != "" {
		dst.TUI.AdminBaseURL = v
	}
	if v := strings.TrimSpace(src.TUI.RefreshInterval); v != "" {
		dst.TUI.RefreshInterval = v
	}
}

func toSettings(path string, cfg fileConfig) (Settings, error) {
	maxIdle, err := time.ParseDuration(cfg.Daemon.ClientMaxIdle)
	if err != nil {
		return Settings{}, fmt.Errorf("invalid daemon.client_max_idle duration: %w", err)
	}
	refresh, err := time.ParseDuration(cfg.TUI.RefreshInterval)
	if err != nil {
		return Settings{}, fmt.Errorf("invalid tui.refresh_interval duration: %w", err)
	}
	return Settings{
		Path:               path,
		DaemonAddr:         cfg.Daemon.Addr,
		MCPToken:           cfg.Auth.MCPToken,
		AdminToken:         cfg.Auth.AdminToken,
		ClientMaxIdle:      maxIdle,
		AdminBaseURL:       cfg.TUI.AdminBaseURL,
		TUIRefreshInterval: refresh,
	}, nil
}

func writeConfig(path string, cfg fileConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open config file: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString("# SurfingBro config for mcpd and mpcd-tui\n\n"); err != nil {
		return fmt.Errorf("write config header: %w", err)
	}
	if err := toml.NewEncoder(file).Encode(cfg); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return nil
}

func deriveAdminBaseURL(addr string) string {
	host := strings.TrimSpace(addr)
	if host == "" {
		host = defaultDaemonAddr
	}
	if strings.Contains(host, "://") {
		return strings.TrimRight(host, "/")
	}
	if strings.HasPrefix(host, ":") {
		return "http://127.0.0.1" + host
	}
	h, p, err := net.SplitHostPort(host)
	if err == nil {
		if h == "" || h == "0.0.0.0" || h == "::" || h == "[::]" {
			h = "127.0.0.1"
		}
		return "http://" + net.JoinHostPort(h, p)
	}
	if strings.Contains(host, ":") {
		return "http://" + host
	}
	return "http://" + net.JoinHostPort(host, "9099")
}

func randomToken() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}

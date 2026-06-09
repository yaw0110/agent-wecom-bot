package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config holds all application configuration.
type Config struct {
	Wecom    WecomConfig    `toml:"wecom"`
	Agent    AgentConfig    `toml:"agent"`
	Session  SessionConfig  `toml:"session"`
	Pool     PoolConfig     `toml:"pool"`
	Behavior BehaviorConfig `toml:"behavior"`
}

type WecomConfig struct {
	Mode      string `toml:"mode"`       // websocket
	BotID     string `toml:"bot_id"`     // 企业微信智能机器人 ID
	BotSecret string `toml:"bot_secret"` // 企业微信智能机器人 Secret
}

type AgentConfig struct {
	Type           string   `toml:"type"`            // claudecode
	WorkDir        string   `toml:"work_dir"`        // Claude 子进程工作目录
	Model          string   `toml:"model"`           // 模型名称
	APIBaseURL     string   `toml:"api_base_url"`    // 中转站 API 地址 (ANTHROPIC_BASE_URL)
	APIKey         string   `toml:"api_key"`         // 中转站 API Key (ANTHROPIC_API_KEY)
	AllowedTools   []string `toml:"allowed_tools"`   // 允许的工具列表
	PermissionMode string   `toml:"permission_mode"` // bypassPermissions
	SystemPrompt   string   `toml:"system_prompt"`   // 系统提示词
}

type SessionConfig struct {
	TTLMinutes          int `toml:"ttl_minutes"`
	CleanupIntervalMinutes int `toml:"cleanup_interval_minutes"`
}

type PoolConfig struct {
	MaxConcurrent  int `toml:"max_concurrent"`
	MaxIdleSeconds int `toml:"max_idle_seconds"`
}

type BehaviorConfig struct {
	AllowedUsers          []string `toml:"allowed_users"`
	MessageTimeoutSeconds int      `toml:"message_timeout_seconds"`
	MaxTurnsPerSession    int      `toml:"max_turns_per_session"`
	PushPort              int      `toml:"push_port"`               // 本地推送 API 端口
}

// DefaultConfig returns configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Wecom: WecomConfig{
			Mode: "websocket",
		},
		Agent: AgentConfig{
			Type:           "claudecode",
			Model:          "claude-sonnet-4-6",
			AllowedTools:   []string{"Bash", "Read", "Write", "Edit", "Grep", "Glob", "WebFetch"},
			PermissionMode: "bypassPermissions",
		},
		Session: SessionConfig{
			TTLMinutes:          30,
			CleanupIntervalMinutes: 5,
		},
		Pool: PoolConfig{
			MaxConcurrent:  10,
			MaxIdleSeconds: 1800,
		},
		Behavior: BehaviorConfig{
			MessageTimeoutSeconds: 120,
			MaxTurnsPerSession:    20,
			PushPort:              9090,
		},
	}
}

// Load loads configuration from a TOML file, then overrides with environment variables.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	// Load from TOML file
	if path != "" {
		if _, err := toml.DecodeFile(path, cfg); err != nil {
			return nil, fmt.Errorf("failed to load config file %s: %w", path, err)
		}
	}

	// Override with environment variables
	if v := os.Getenv("WECOM_BOT_ID"); v != "" {
		cfg.Wecom.BotID = v
	}
	if v := os.Getenv("WECOM_BOT_SECRET"); v != "" {
		cfg.Wecom.BotSecret = v
	}
	if v := os.Getenv("AGENT_MODEL"); v != "" {
		cfg.Agent.Model = v
	}
	if v := os.Getenv("AGENT_WORK_DIR"); v != "" {
		cfg.Agent.WorkDir = v
	}
	if v := os.Getenv("AGENT_SYSTEM_PROMPT"); v != "" {
		cfg.Agent.SystemPrompt = v
	}
	if v := os.Getenv("AGENT_API_BASE_URL"); v != "" {
		cfg.Agent.APIBaseURL = v
	}
	if v := os.Getenv("AGENT_API_KEY"); v != "" {
		cfg.Agent.APIKey = v
	}
	if v := os.Getenv("POOL_MAX_CONCURRENT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Pool.MaxConcurrent = n
		}
	}
	if v := os.Getenv("SESSION_TTL_MINUTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Session.TTLMinutes = n
		}
	}

	// Validate
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	var errs []string

	if c.Wecom.BotID == "" {
		errs = append(errs, "wecom.bot_id is required (set in config or WECOM_BOT_ID env)")
	}
	if c.Wecom.BotSecret == "" {
		errs = append(errs, "wecom.bot_secret is required (set in config or WECOM_BOT_SECRET env)")
	}
	if c.Agent.Type != "claudecode" {
		errs = append(errs, "agent.type must be 'claudecode'")
	}
	if c.Agent.WorkDir == "" {
		errs = append(errs, "agent.work_dir is required")
	}
	if c.Pool.MaxConcurrent <= 0 {
		errs = append(errs, "pool.max_concurrent must be > 0")
	}
	if c.Session.TTLMinutes <= 0 {
		errs = append(errs, "session.ttl_minutes must be > 0")
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuration validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

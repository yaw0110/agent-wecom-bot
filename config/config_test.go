package config

import (
	"os"
	"testing"
)

func TestLoad_DefaultConfig(t *testing.T) {
	// Without env vars, config should fail validation (missing required fields)
	_, err := Load("")
	if err == nil {
		t.Fatal("expected validation error without required fields")
	}
}

func TestLoad_WithEnvVars(t *testing.T) {
	os.Setenv("WECOM_BOT_ID", "test-bot-id")
	os.Setenv("WECOM_BOT_SECRET", "test-secret")
	os.Setenv("AGENT_WORK_DIR", "/tmp/test-workdir")
	defer func() {
		os.Unsetenv("WECOM_BOT_ID")
		os.Unsetenv("WECOM_BOT_SECRET")
		os.Unsetenv("AGENT_WORK_DIR")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Wecom.BotID != "test-bot-id" {
		t.Errorf("expected test-bot-id, got %s", cfg.Wecom.BotID)
	}
	if cfg.Wecom.BotSecret != "test-secret" {
		t.Errorf("expected test-secret, got %s", cfg.Wecom.BotSecret)
	}
	if cfg.Agent.WorkDir != "/tmp/test-workdir" {
		t.Errorf("expected /tmp/test-workdir, got %s", cfg.Agent.WorkDir)
	}
	if cfg.Agent.Model != "claude-sonnet-4-6" {
		t.Errorf("expected claude-sonnet-4-6, got %s", cfg.Agent.Model)
	}
}

func TestValidate_MissingFields(t *testing.T) {
	cfg := DefaultConfig()
	err := cfg.validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidate_AllFields(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Wecom.BotID = "bot-id"
	cfg.Wecom.BotSecret = "secret"
	cfg.Agent.WorkDir = "/tmp/work"
	err := cfg.validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Pool.MaxConcurrent != 10 {
		t.Errorf("expected 10, got %d", cfg.Pool.MaxConcurrent)
	}
	if cfg.Session.TTLMinutes != 30 {
		t.Errorf("expected 30, got %d", cfg.Session.TTLMinutes)
	}
	if cfg.Behavior.MessageTimeoutSeconds != 120 {
		t.Errorf("expected 120, got %d", cfg.Behavior.MessageTimeoutSeconds)
	}
	if cfg.Agent.Model != "claude-sonnet-4-6" {
		t.Errorf("expected claude-sonnet-4-6, got %s", cfg.Agent.Model)
	}
}

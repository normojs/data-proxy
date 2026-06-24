package dpagent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckServiceStatusLinuxActive(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	configPath := writeTestServiceConfig(t)
	def, err := BuildServiceDefinition(ServiceOptions{
		ConfigPath: configPath,
		BinaryPath: "/usr/local/bin/data-proxy-agent",
		Platform:   "linux",
		Scope:      "user",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(def.InstallPath), DefaultConfigFolderMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(def.InstallPath, []byte("[Unit]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var gotName string
	var gotArgs []string
	check := CheckServiceStatus(context.Background(), ServiceHealthOptions{
		ConfigPath: configPath,
		BinaryPath: "/usr/local/bin/data-proxy-agent",
		Platform:   "linux",
		Scope:      "user",
		CommandExec: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			gotName = name
			gotArgs = append([]string(nil), args...)
			return []byte("active\n"), nil
		},
	})
	if check.Status != HealthStatusOK {
		t.Fatalf("expected ok service status: %#v", check)
	}
	if gotName != "systemctl" || strings.Join(gotArgs, " ") != "--user is-active data-proxy-agent.service" {
		t.Fatalf("unexpected service status command: %s %s", gotName, strings.Join(gotArgs, " "))
	}
}

func TestCheckServiceStatusLinuxNotInstalledWarns(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	configPath := writeTestServiceConfig(t)
	called := false
	check := CheckServiceStatus(context.Background(), ServiceHealthOptions{
		ConfigPath: configPath,
		BinaryPath: "/usr/local/bin/data-proxy-agent",
		Platform:   "linux",
		Scope:      "user",
		CommandExec: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			called = true
			return nil, nil
		},
	})
	if check.Status != HealthStatusWarn || !strings.Contains(check.Detail, "not installed") {
		t.Fatalf("expected not installed warning: %#v", check)
	}
	if called {
		t.Fatal("service status command should not run when linux unit is not installed")
	}
}

func TestCheckServiceStatusWindowsRunning(t *testing.T) {
	configPath := writeTestServiceConfig(t)
	check := CheckServiceStatus(context.Background(), ServiceHealthOptions{
		ConfigPath: configPath,
		BinaryPath: `C:\Program Files\DataProxy\data-proxy-agent.exe`,
		Platform:   "windows",
		Scope:      "system",
		CommandExec: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name != "sc.exe" || strings.Join(args, " ") != "query DataProxyAgent" {
				t.Fatalf("unexpected windows service status command: %s %s", name, strings.Join(args, " "))
			}
			return []byte("STATE              : 4  RUNNING\n"), nil
		},
	})
	if check.Status != HealthStatusOK {
		t.Fatalf("expected windows service running: %#v", check)
	}
}

func TestCheckServiceStatusWindowsStoppedWarns(t *testing.T) {
	configPath := writeTestServiceConfig(t)
	check := CheckServiceStatus(context.Background(), ServiceHealthOptions{
		ConfigPath: configPath,
		BinaryPath: `C:\Program Files\DataProxy\data-proxy-agent.exe`,
		Platform:   "windows",
		Scope:      "system",
		CommandExec: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("STATE              : 1  STOPPED\n"), errors.New("exit status 1062")
		},
	})
	if check.Status != HealthStatusWarn || !strings.Contains(check.Detail, "stopped") {
		t.Fatalf("expected windows stopped warning: %#v", check)
	}
}

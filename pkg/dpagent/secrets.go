package dpagent

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	keyring "github.com/zalando/go-keyring"
)

const (
	DefaultSecretStoreService = "data-proxy-agent"
	TokenStoreAuto            = "auto"
	TokenStoreConfig          = "config"
	TokenStoreNative          = "native"
	TokenStoreSecretFile      = "secret-file"
)

type secretRefParts struct {
	Scheme  string
	Service string
	Account string
	Path    string
}

func NormalizeTokenStoreMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", TokenStoreAuto:
		return TokenStoreAuto
	case "config", "plain", "yaml":
		return TokenStoreConfig
	case "native", "keyring", "keychain", "secret-service", "wincred":
		return TokenStoreNative
	case "file", TokenStoreSecretFile, "secret_file":
		return TokenStoreSecretFile
	default:
		return strings.ToLower(strings.TrimSpace(mode))
	}
}

func StoreAgentToken(configPath string, cfg *Config, token string, mode string) (string, error) {
	if cfg == nil {
		return "", errors.New("config is nil")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", errors.New("token is empty")
	}
	mode = NormalizeTokenStoreMode(mode)
	switch mode {
	case TokenStoreConfig:
		cfg.Agent.Token = token
		cfg.Agent.TokenRef = ""
		return "agent.token", nil
	case TokenStoreNative:
		ref := DefaultNativeTokenRef(cfg.Agent.ClientID)
		if err := WriteSecretRef(ref, token); err != nil {
			return "", err
		}
		cfg.Agent.Token = ""
		cfg.Agent.TokenRef = ref
		return ref, nil
	case TokenStoreSecretFile:
		ref, err := DefaultSecretFileTokenRef(configPath)
		if err != nil {
			return "", err
		}
		if err := WriteSecretRef(ref, token); err != nil {
			return "", err
		}
		cfg.Agent.Token = ""
		cfg.Agent.TokenRef = ref
		return ref, nil
	case TokenStoreAuto:
		ref := DefaultNativeTokenRef(cfg.Agent.ClientID)
		if err := WriteSecretRef(ref, token); err == nil {
			cfg.Agent.Token = ""
			cfg.Agent.TokenRef = ref
			return ref, nil
		}
		ref, err := DefaultSecretFileTokenRef(configPath)
		if err != nil {
			return "", err
		}
		if err := WriteSecretRef(ref, token); err != nil {
			return "", err
		}
		cfg.Agent.Token = ""
		cfg.Agent.TokenRef = ref
		return ref, nil
	default:
		return "", fmt.Errorf("unsupported token store mode %q", mode)
	}
}

func DefaultNativeTokenRef(clientID string) string {
	account := strings.TrimSpace(clientID)
	if account == "" {
		account = "default"
	}
	return "keyring://" + url.PathEscape(DefaultSecretStoreService) + "/" + url.PathEscape(account)
}

func DefaultSecretFileTokenRef(configPath string) (string, error) {
	if strings.TrimSpace(configPath) == "" {
		resolved, err := ConfigPath()
		if err != nil {
			return "", err
		}
		configPath = resolved
	}
	configPath = expandPath(configPath)
	if strings.TrimSpace(configPath) == "" {
		return "", errors.New("config path is empty")
	}
	return "secret-file://" + filepath.ToSlash(filepath.Join(filepath.Dir(configPath), "agent.token")), nil
}

func ReadSecretRef(ref string) (string, error) {
	parts, err := parseSecretRef(ref)
	if err != nil {
		return "", err
	}
	switch parts.Scheme {
	case "keyring", "keychain", "native":
		return keyring.Get(parts.Service, parts.Account)
	case "secret-file", "file":
		bytes, err := os.ReadFile(parts.Path)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(bytes)), nil
	default:
		return "", fmt.Errorf("unsupported secret ref scheme %q", parts.Scheme)
	}
}

func WriteSecretRef(ref string, value string) error {
	parts, err := parseSecretRef(ref)
	if err != nil {
		return err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("secret value is empty")
	}
	switch parts.Scheme {
	case "keyring", "keychain", "native":
		return keyring.Set(parts.Service, parts.Account, value)
	case "secret-file", "file":
		if err := os.MkdirAll(filepath.Dir(parts.Path), DefaultConfigFolderMode); err != nil {
			return err
		}
		return os.WriteFile(parts.Path, []byte(value+"\n"), DefaultConfigFileMode)
	default:
		return fmt.Errorf("unsupported secret ref scheme %q", parts.Scheme)
	}
}

func DeleteSecretRef(ref string) error {
	parts, err := parseSecretRef(ref)
	if err != nil {
		return err
	}
	switch parts.Scheme {
	case "keyring", "keychain", "native":
		if err := keyring.Delete(parts.Service, parts.Account); err != nil {
			return err
		}
		return nil
	case "secret-file", "file":
		if err := os.Remove(parts.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported secret ref scheme %q", parts.Scheme)
	}
}

func parseSecretRef(ref string) (secretRefParts, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return secretRefParts{}, errors.New("secret ref is empty")
	}
	u, err := url.Parse(ref)
	if err != nil {
		return secretRefParts{}, err
	}
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme == "" {
		return secretRefParts{}, errors.New("secret ref scheme is required")
	}
	switch scheme {
	case "keyring", "keychain", "native":
		service, err := url.PathUnescape(u.Host)
		if err != nil {
			return secretRefParts{}, err
		}
		account := strings.TrimPrefix(u.EscapedPath(), "/")
		account, err = url.PathUnescape(account)
		if err != nil {
			return secretRefParts{}, err
		}
		if strings.TrimSpace(service) == "" || strings.TrimSpace(account) == "" {
			return secretRefParts{}, errors.New("keyring ref must be keyring://<service>/<account>")
		}
		return secretRefParts{Scheme: scheme, Service: service, Account: account}, nil
	case "secret-file", "file":
		path := u.Path
		if u.Host != "" && path == "" {
			path = u.Host
		} else if u.Host != "" {
			path = string(filepath.Separator) + filepath.Join(u.Host, path)
		}
		path = expandPath(path)
		if strings.TrimSpace(path) == "" {
			return secretRefParts{}, errors.New("secret-file ref path is required")
		}
		return secretRefParts{Scheme: scheme, Path: filepath.Clean(path)}, nil
	default:
		return secretRefParts{}, fmt.Errorf("unsupported secret ref scheme %q", scheme)
	}
}

func tokenFromInput(value string, envName string, readStdin bool, stdin io.Reader) (string, error) {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value), nil
	}
	if strings.TrimSpace(envName) != "" {
		token := strings.TrimSpace(os.Getenv(strings.TrimSpace(envName)))
		if token == "" {
			return "", fmt.Errorf("environment variable %s is empty", strings.TrimSpace(envName))
		}
		return token, nil
	}
	if readStdin {
		if stdin == nil {
			stdin = os.Stdin
		}
		bytes, err := io.ReadAll(io.LimitReader(stdin, 64*1024))
		if err != nil {
			return "", err
		}
		token := strings.TrimSpace(string(bytes))
		if token == "" {
			return "", errors.New("stdin token is empty")
		}
		return token, nil
	}
	return "", errors.New("token value is required; pass --value, --value-env, or --stdin")
}

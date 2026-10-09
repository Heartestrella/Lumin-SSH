package codexbridge

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultBaseURL is the legacy sentinel stored in existing settings. At runtime
	// it is replaced by the random loopback address of the built-in gateway.
	DefaultBaseURL = "http://127.0.0.1:5050/v1"
	installURL     = "https://developers.openai.com/codex/cli/"
)

type Config struct {
	Enabled          bool
	BaseURL          string
	ExecutablePath   string
	WorkingDirectory string
}
type Status struct {
	State          string `json:"state"`
	Enabled        bool   `json:"enabled"`
	Managed        bool   `json:"managed"`
	BaseURL        string `json:"baseUrl"`
	ExecutablePath string `json:"executablePath,omitempty"`
	Message        string `json:"message,omitempty"`
	InstallURL     string `json:"installUrl"`
}

type Manager struct {
	mu         sync.Mutex
	opMu       sync.Mutex
	config     Config
	status     Status
	generation uint64
	client     *http.Client
	lookPath   func(string) (string, error)
	stat       func(string) (os.FileInfo, error)
	homeDir    func() (string, error)
	newGateway func(string, string) *Gateway
	gateway    *Gateway
	resolver   SessionContextResolver
}

func NewManager() *Manager {
	return &Manager{
		status:   Status{State: "stopped", BaseURL: DefaultBaseURL, InstallURL: installURL},
		client:   &http.Client{Timeout: 1500 * time.Millisecond},
		lookPath: exec.LookPath, stat: os.Stat, homeDir: os.UserHomeDir,
		newGateway: func(executable, workingDir string) *Gateway {
			return NewGateway(nativeExecRunner{executable: executable}, workingDir, 2)
		},
	}
}

func NormalizeBaseURL(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return DefaultBaseURL
	}
	return value
}
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}
func (m *Manager) BaseURL() string { return m.Status().BaseURL }

// SetSessionContextResolver supplies terminal working directories to every
// managed gateway, including gateways created by a later Apply or Refresh.
func (m *Manager) SetSessionContextResolver(resolve SessionContextResolver) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.resolver = resolve
	gateway := m.gateway
	m.mu.Unlock()
	if gateway != nil {
		gateway.SetSessionContextResolver(resolve)
	}
}

func (m *Manager) Apply(c Config) {
	if m == nil {
		return
	}
	c.BaseURL = NormalizeBaseURL(c.BaseURL)
	c.ExecutablePath = strings.TrimSpace(c.ExecutablePath)
	c.WorkingDirectory = strings.TrimSpace(c.WorkingDirectory)
	m.mu.Lock()
	previous, previousConfig := m.status, m.config
	active := previous.State == "starting" || previous.State == "running" || previous.State == "external"
	if previousConfig == c && (active || (!c.Enabled && previous.State == "stopped")) {
		m.mu.Unlock()
		return
	}
	m.config = c
	m.generation++
	generation := m.generation
	m.status = Status{State: "stopped", Enabled: c.Enabled, BaseURL: c.BaseURL, ExecutablePath: c.ExecutablePath, InstallURL: installURL}
	if c.Enabled {
		m.status.State, m.status.Message = "starting", "正在启动 Codex 内置网关…"
	}
	m.mu.Unlock()
	go func() {
		m.opMu.Lock()
		defer m.opMu.Unlock()
		m.closeGateway()
		if c.Enabled {
			m.ensure(generation, c)
		}
	}()
}

func (m *Manager) Refresh() {
	if m == nil {
		return
	}
	m.mu.Lock()
	c := m.config
	m.generation++
	generation := m.generation
	if c.Enabled {
		m.status.State, m.status.Message = "starting", "正在重新启动 Codex 内置网关…"
	}
	m.mu.Unlock()
	if c.Enabled {
		go func() {
			m.opMu.Lock()
			defer m.opMu.Unlock()
			m.closeGateway()
			m.ensure(generation, c)
		}()
	}
}

func (m *Manager) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.generation++
	m.status = Status{State: "stopped", BaseURL: DefaultBaseURL, InstallURL: installURL}
	m.mu.Unlock()
	m.opMu.Lock()
	defer m.opMu.Unlock()
	m.closeGateway()
}

func (m *Manager) ensure(generation uint64, c Config) {
	// A non-default URL is an explicit external OpenAI-compatible endpoint.
	if c.BaseURL != DefaultBaseURL {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		healthy := m.healthy(ctx, c.BaseURL)
		cancel()
		if healthy {
			m.finish(generation, Status{State: "external", Enabled: true, BaseURL: c.BaseURL, InstallURL: installURL, Message: "检测到外部 OpenAI 兼容 Codex 网关"})
		} else {
			m.finish(generation, Status{State: "error", Enabled: true, BaseURL: c.BaseURL, InstallURL: installURL, Message: "自定义 Codex 网关不可用：" + c.BaseURL})
		}
		return
	}
	executable, err := m.findExecutable(c.ExecutablePath)
	if err != nil {
		m.finish(generation, Status{State: "not_installed", Enabled: true, BaseURL: c.BaseURL, InstallURL: installURL, Message: "未找到 Codex CLI，请安装 Codex CLI 或填写可执行文件路径"})
		log.Printf("codex gateway: executable not found: %v", err)
		return
	}
	gateway := m.newGateway(executable, c.WorkingDirectory)
	m.mu.Lock()
	resolver := m.resolver
	m.mu.Unlock()
	gateway.SetSessionContextResolver(resolver)
	baseURL, err := gateway.Start()
	if err != nil {
		m.finish(generation, Status{State: "error", Enabled: true, BaseURL: c.BaseURL, ExecutablePath: executable, InstallURL: installURL, Message: "Codex 内置网关启动失败：" + err.Error()})
		return
	}
	m.mu.Lock()
	if generation != m.generation {
		m.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = gateway.Close(ctx)
		cancel()
		return
	}
	// A resolver may have been replaced while the gateway was starting.
	gateway.SetSessionContextResolver(m.resolver)
	m.gateway = gateway
	m.status = Status{State: "running", Enabled: true, Managed: true, BaseURL: baseURL, ExecutablePath: executable, InstallURL: installURL, Message: "内置 HTTP 网关运行中（Codex exec）"}
	m.mu.Unlock()
}

func (m *Manager) healthy(ctx context.Context, baseURL string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, NormalizeBaseURL(baseURL)+"/models", nil)
	if err != nil {
		return false
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func (m *Manager) findExecutable(configured string) (string, error) {
	if configured != "" {
		if info, err := m.stat(configured); err == nil && !info.IsDir() {
			return configured, nil
		}
		return "", fmt.Errorf("configured executable does not exist: %s", configured)
	}
	if found, err := m.lookPath("codex"); err == nil && found != "" {
		return found, nil
	}
	home, _ := m.homeDir()
	names := []string{"codex"}
	if runtime.GOOS == "windows" {
		names = []string{"codex.exe", "codex.cmd"}
	}
	for _, name := range names {
		for _, path := range []string{filepath.Join(home, ".local", "bin", name), filepath.Join(home, "AppData", "Roaming", "npm", name), filepath.Join(home, "scoop", "shims", name)} {
			if info, err := m.stat(path); err == nil && !info.IsDir() {
				return path, nil
			}
		}
	}
	return "", errors.New("codex executable not found")
}

func (m *Manager) finish(generation uint64, status Status) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if generation == m.generation {
		m.status = status
	}
}
func (m *Manager) closeGateway() {
	m.mu.Lock()
	gateway := m.gateway
	m.gateway = nil
	m.mu.Unlock()
	if gateway != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if err := gateway.Close(ctx); err != nil {
			log.Printf("codex gateway: stop failed: %v", err)
		}
		cancel()
	}
}

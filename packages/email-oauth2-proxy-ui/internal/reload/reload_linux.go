//go:build linux

package reload

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"email-oauth2-proxy-ui/internal/logs"
)

// Result describes the outcome of a proxy container control request.
type Result struct {
	Supported bool   `json:"supported"`
	Reloaded  bool   `json:"reloaded"`
	Message   string `json:"message"`
}

// CanRestart reports whether the Docker Unix socket is mounted and accessible.
func CanRestart(socketPath, containerName string) bool {
	if socketPath == "" || strings.TrimSpace(containerName) == "" {
		return false
	}
	info, err := os.Stat(socketPath)
	return err == nil && info.Mode()&os.ModeSocket != 0
}

// CanToggleDebug reports whether we can send SIGUSR1 to emailproxy.py (via shared /proc or Docker Exec API).
func CanToggleDebug(socketPath, containerName string) bool {
	if findEmailProxyPID() > 0 {
		return true
	}
	return CanRestart(socketPath, containerName)
}

// DetectDebugState scans recent lines of emailproxy.log to infer whether Debug Mode is currently enabled.
func DetectDebugState(logPath string) (enabled bool, known bool) {
	lines, _, err := logs.ReadTail(logPath, 500)
	if err != nil {
		return false, false
	}
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if strings.Contains(line, "Setting debug mode: True") {
			return true, true
		}
		if strings.Contains(line, "Setting debug mode: False") {
			return false, true
		}
		if strings.Contains(line, "Initialising Email OAuth 2.0 Proxy") {
			return strings.Contains(line, "in debug mode"), true
		}
	}
	return false, false
}

// ToggleDebug sends SIGUSR1 to emailproxy.py to toggle Debug Mode on/off at runtime without restarting.
func ToggleDebug(socketPath, containerName string) Result {
	// 1. Direct /proc signaling if sharing PID namespace
	if pid := findEmailProxyPID(); pid > 0 {
		if err := syscall.Kill(pid, syscall.SIGUSR1); err == nil {
			return Result{
				Supported: true,
				Reloaded:  true,
				Message:   fmt.Sprintf("Debug mode alternado em tempo real (SIGUSR1 enviado ao PID %d).", pid),
			}
		}
	}

	// 2. Docker Engine Exec API via /var/run/docker.sock
	containerName = strings.TrimSpace(containerName)
	if !CanRestart(socketPath, containerName) {
		return Result{
			Supported: false,
			Reloaded:  false,
			Message:   "Monte /var/run/docker.sock no compose para alternar o Debug Mode em tempo real pelo painel.",
		}
	}

	client := dockerHTTPClient(socketPath, 15*time.Second)

	execPayload, _ := json.Marshal(map[string]any{
		"AttachStdout": true,
		"AttachStderr": true,
		"Cmd":          []string{"sh", "-c", "pkill -USR1 -f emailproxy.py || kill -USR1 $(pidof python)"},
	})
	createURL := fmt.Sprintf("http://docker/containers/%s/exec", url.PathEscape(containerName))
	req, err := http.NewRequest(http.MethodPost, createURL, bytes.NewReader(execPayload))
	if err != nil {
		return Result{Supported: true, Message: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return Result{Supported: true, Message: fmt.Sprintf("Falha ao falar com o Docker socket: %v", err)}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 400 {
		return Result{Supported: true, Message: fmt.Sprintf("Docker exec create HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))}
	}

	var created struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(body, &created); err != nil || created.ID == "" {
		return Result{Supported: true, Message: "Resposta inválida ao criar exec no Docker."}
	}

	startURL := fmt.Sprintf("http://docker/exec/%s/start", url.PathEscape(created.ID))
	startReq, _ := http.NewRequest(http.MethodPost, startURL, strings.NewReader(`{"Detach":false,"Tty":false}`))
	startReq.Header.Set("Content-Type", "application/json")
	startResp, err := client.Do(startReq)
	if err != nil {
		return Result{Supported: true, Message: fmt.Sprintf("Falha ao executar SIGUSR1: %v", err)}
	}
	defer startResp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(startResp.Body, 4096))
	if startResp.StatusCode >= 400 {
		return Result{Supported: true, Message: fmt.Sprintf("Docker exec start HTTP %d", startResp.StatusCode)}
	}

	return Result{
		Supported: true,
		Reloaded:  true,
		Message:   "Debug mode alternado em tempo real (SIGUSR1 enviado ao emailproxy.py).",
	}
}

// RestartProxy restarts the email-oauth2-proxy container via the Docker Engine API over Unix socket.
// Note: We intentionally avoid sending SIGHUP to emailproxy.py because upstream email-oauth2-proxy
// hits OSError(98, 'Address in use') and exits when reloading SSL/TLS listening sockets via SIGHUP.
func RestartProxy(socketPath, containerName string) Result {
	containerName = strings.TrimSpace(containerName)
	if !CanRestart(socketPath, containerName) {
		return Result{
			Supported: false,
			Reloaded:  false,
			Message:   "Reinicie o container do Email OAuth2 Proxy pelo painel do Runtipi para aplicar (ou monte /var/run/docker.sock para reiniciar em 1 clique pelo painel).",
		}
	}

	client := dockerHTTPClient(socketPath, 25*time.Second)

	endpoint := fmt.Sprintf("http://docker/containers/%s/restart?t=5", url.PathEscape(containerName))
	req, err := http.NewRequest(http.MethodPost, endpoint, nil)
	if err != nil {
		return Result{Supported: true, Reloaded: false, Message: fmt.Sprintf("Erro ao montar requisição: %v", err)}
	}

	resp, err := client.Do(req)
	if err != nil {
		return Result{
			Supported: true,
			Reloaded:  false,
			Message:   fmt.Sprintf("Falha ao falar com o Docker socket: %v", err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return Result{
			Supported: true,
			Reloaded:  true,
			Message:   fmt.Sprintf("Container %s reiniciado com sucesso!", containerName),
		}
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return Result{
		Supported: true,
		Reloaded:  false,
		Message:   fmt.Sprintf("Docker respondeu HTTP %d para container '%s': %s", resp.StatusCode, containerName, strings.TrimSpace(string(body))),
	}
}

func dockerHTTPClient(socketPath string, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", socketPath)
			},
		},
	}
}

func findEmailProxyPID() int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 1 {
			continue
		}
		cmdline, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		if err != nil {
			continue
		}
		cmd := strings.ReplaceAll(string(cmdline), "\x00", " ")
		if strings.Contains(cmd, "emailproxy.py") {
			return pid
		}
	}
	return 0
}

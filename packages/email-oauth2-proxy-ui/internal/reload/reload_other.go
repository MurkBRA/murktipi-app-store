//go:build !linux

package reload

import (
	"strings"

	"email-oauth2-proxy-ui/internal/logs"
)

// Result describes the outcome of a proxy container control request.
type Result struct {
	Supported bool   `json:"supported"`
	Reloaded  bool   `json:"reloaded"`
	Message   string `json:"message"`
}

// CanRestart reports whether container restart via Unix socket is supported.
func CanRestart(_, _ string) bool {
	return false
}

// CanToggleDebug reports whether live SIGUSR1 debug toggling is supported.
func CanToggleDebug(_, _ string) bool {
	return false
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

// ToggleDebug returns unsupported on non-Linux hosts.
func ToggleDebug(_, _ string) Result {
	return Result{
		Supported: false,
		Reloaded:  false,
		Message:   "Alternar Debug Mode em tempo real (SIGUSR1) só está disponível em Linux/Docker.",
	}
}

// RestartProxy returns unsupported on non-Linux hosts.
func RestartProxy(_, _ string) Result {
	return Result{
		Supported: false,
		Reloaded:  false,
		Message:   "Reinicie o container do Email OAuth2 Proxy pelo painel do Runtipi para aplicar as alterações.",
	}
}

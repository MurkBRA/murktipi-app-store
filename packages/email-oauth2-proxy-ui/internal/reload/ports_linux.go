//go:build linux

package reload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"email-oauth2-proxy-ui/internal/config"
)

type dockerContainerSummary struct {
	ID     string   `json:"Id"`
	Names  []string `json:"Names"`
	State  string   `json:"State"`
	Status string   `json:"Status"`
	Ports  []struct {
		IP          string `json:"IP"`
		PrivatePort int    `json:"PrivatePort"`
		PublicPort  int    `json:"PublicPort"`
		Type        string `json:"Type"`
	} `json:"Ports"`
}

type dockerContainerInspect struct {
	ID      string `json:"Id"`
	Name    string `json:"Name"`
	Image   string `json:"Image"`
	State   struct {
		Running bool `json:"Running"`
	} `json:"State"`
	Config          map[string]any `json:"Config"`
	HostConfig      map[string]any `json:"HostConfig"`
	NetworkSettings struct {
		Networks map[string]struct {
			NetworkID  string         `json:"NetworkID"`
			Gateway    string         `json:"Gateway"`
			Aliases    []string       `json:"Aliases"`
			IPAMConfig map[string]any `json:"IPAMConfig"`
			Links      []string       `json:"Links"`
			DriverOpts map[string]any `json:"DriverOpts"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
}

// InspectDockerPortsOverview gathers live Docker published ports, Compose file status, and per-port conflict info.
// This is fast and non-invasive (does not spawn temporary probe containers).
func InspectDockerPortsOverview(socketPath, containerName, composePath, composeService string, servers []config.ServerEntry) DockerPortsOverview {
	specs := SpecsFromServers(servers)
	resolvedCompose, composeWritable := ResolveComposeFile(composePath)

	out := DockerPortsOverview{
		DockerAvailable:  CanRestart(socketPath, containerName),
		ComposeAvailable: composeWritable,
		ComposePath:      resolvedCompose,
		ComposeService:   composeService,
		PortsYAML:        RenderPortsSnippet(specs),
		PublishedPorts:   []int{},
		ProtectedPorts:   []int{},
		Statuses:         []PortStatus{},
	}

	if !out.DockerAvailable {
		out.Statuses = evaluatePortsStatic(servers, nil, nil, "")
		return out
	}

	client := dockerHTTPClient(socketPath, 8*time.Second)

	// Check if UI container itself is bound to PidMode: container:...
	out.SelfPidShared = inspectSelfPidShared(client)

	proxyInspect, err := inspectContainer(client, containerName)
	if err != nil {
		out.Statuses = evaluatePortsStatic(servers, nil, nil, "")
		return out
	}

	out.ProxyFound = true
	out.ProxyRunning = proxyInspect.State.Running

	if labels, ok := proxyInspect.Config["Labels"].(map[string]any); ok {
		if cf, ok := labels["com.docker.compose.project.config_files"].(string); ok && cf != "" {
			parts := strings.Split(cf, ",")
			out.ComposeHostHint = strings.TrimSpace(parts[len(parts)-1])
		}
		if out.ComposeService == "" {
			if svc, ok := labels["com.docker.compose.service"].(string); ok && svc != "" {
				out.ComposeService = svc
			}
		}
	}
	if out.ComposeService == "" {
		out.ComposeService = "email-oauth2-proxy"
	}

	proxyPorts, protectedPorts := extractProxyPortBindings(proxyInspect)
	out.PublishedPorts = sortedKeysInt(proxyPorts)
	out.ProtectedPorts = sortedKeysInt(protectedPorts)

	otherPorts := listOtherDockerHostPorts(client, proxyInspect.ID)
	gatewayIP := pickGatewayIP(proxyInspect)

	// Build full server list including any extra non-80 ports published on proxy that aren't in emailproxy.config yet
	statuses := evaluatePortsStatic(servers, proxyPorts, otherPorts, gatewayIP)
	inConfig := make(map[int]bool, len(servers))
	for _, s := range servers {
		inConfig[s.LocalPort] = true
	}
	for _, p := range out.PublishedPorts {
		if inConfig[p] {
			continue
		}
		if protectedPorts[p] {
			statuses = append(statuses, PortStatus{
				Port:             p,
				Section:          "Callback OAuth (porta interna 80)",
				Protocol:         "HTTP",
				PublishedOnProxy: true,
				Protected:        true,
				Status:           "open_on_proxy",
				Message:          "Protegida pelo Runtipi (callback OAuth na porta interna 80)",
			})
			continue
		}
		statuses = append(statuses, PortStatus{
			Port:             p,
			Section:          fmt.Sprintf("EXTRA-%d", p),
			PublishedOnProxy: true,
			Status:           "open_on_proxy",
			Message:          "Aberta no Docker (não listada no emailproxy.config)",
		})
	}
	out.Statuses = statuses
	return out
}

// CheckPortsWithHostProbe verifies availability of candidatePorts, including a real Docker host-bind probe
// for any port that is not already published by email-oauth2-proxy.
func CheckPortsWithHostProbe(socketPath, containerName string, servers []config.ServerEntry, candidatePorts []int) ([]PortStatus, error) {
	if !CanRestart(socketPath, containerName) {
		return evaluatePortsStatic(servers, nil, nil, ""), fmt.Errorf("Docker socket não está disponível em %s", socketPath)
	}

	client := dockerHTTPClient(socketPath, 10*time.Second)
	proxyInspect, err := inspectContainer(client, containerName)
	if err != nil {
		return nil, fmt.Errorf("container '%s' não encontrado no Docker: %w", containerName, err)
	}

	proxyPorts, _ := extractProxyPortBindings(proxyInspect)
	otherPorts := listOtherDockerHostPorts(client, proxyInspect.ID)
	gatewayIP := pickGatewayIP(proxyInspect)

	// Merge servers with any explicit candidatePorts not yet in emailproxy.config
	combined := append([]config.ServerEntry(nil), servers...)
	seenPort := make(map[int]bool)
	for _, s := range combined {
		seenPort[s.LocalPort] = true
	}
	for _, p := range candidatePorts {
		if p > 0 && !seenPort[p] {
			seenPort[p] = true
			combined = append(combined, config.ServerEntry{
				Section:   fmt.Sprintf("PORT-%d", p),
				LocalPort: p,
			})
		}
	}

	statuses := evaluatePortsStatic(combined, proxyPorts, otherPorts, gatewayIP)

	// Determine probe image (prefer our own container image, fallback to proxy's image)
	probeImage := selfContainerImage(client)
	if probeImage == "" {
		probeImage = proxyInspect.Image
	}

	// Run real host-bind probe only on ports marked "available" and included in candidatePorts (or all candidatePorts if empty)
	targetSet := make(map[int]bool)
	for _, p := range candidatePorts {
		targetSet[p] = true
	}
	for i := range statuses {
		st := &statuses[i]
		if st.Status != "available" {
			continue
		}
		if len(targetSet) > 0 && !targetSet[st.Port] {
			continue
		}
		if busy, msg := probeHostPortViaDockerBind(client, probeImage, st.Port); busy {
			st.Status = "conflict_host"
			st.Message = msg
		} else if msg != "" {
			st.Message = msg
		}
	}

	return statuses, nil
}

// ApplyPorts executes the user's chosen port operation:
// - mode == "compose_only": updates COMPOSE_FILE only
// - mode == "docker_only": checks new ports & safely recreates container with rollback
// - mode == "compose_and_docker": checks new ports, updates COMPOSE_FILE (.bak), and safely recreates container with rollback
func ApplyPorts(socketPath, containerName, composePath, composeService, mode string, specs []PortSpec) ApplyPortsResult {
	specs = DeduplicateAndSortSpecs(specs)
	resolvedCompose, composeWritable := ResolveComposeFile(composePath)

	wantCompose := mode == "compose_only" || mode == "compose_and_docker"
	wantDocker := mode == "docker_only" || mode == "compose_and_docker"

	if !wantCompose && !wantDocker {
		return ApplyPortsResult{Mode: mode, Message: "Modo inválido (use docker_only, compose_and_docker ou compose_only)."}
	}

	if wantCompose && !composeWritable {
		return ApplyPortsResult{
			Mode:    mode,
			Message: "Arquivo docker-compose.yml não está montado com permissão de escrita em /compose. Use o modo 'Abrir no Docker agora' ou monte a pasta user-config em :/compose:rw.",
		}
	}

	var client *http.Client
	var proxyInspect *dockerContainerInspect
	var proxyPorts map[int]bool
	var protectedBindings map[string][]map[string]string
	var newPorts []int

	if wantDocker {
		if !CanRestart(socketPath, containerName) {
			return ApplyPortsResult{Mode: mode, Message: "Docker socket não está montado em /var/run/docker.sock."}
		}
		client = dockerHTTPClient(socketPath, 30*time.Second)

		if inspectSelfPidShared(client) {
			return ApplyPortsResult{
				Mode:    mode,
				Message: "O container do painel ainda está usando 'pid: service:email-oauth2-proxy' no docker-compose.yml. Remova essa linha e reinicie pelo Runtipi antes de abrir portas automaticamente (senão o painel cairia ao recriar o proxy).",
			}
		}

		var err error
		proxyInspect, err = inspectContainer(client, containerName)
		if err != nil {
			return ApplyPortsResult{Mode: mode, Message: fmt.Sprintf("Container '%s' não encontrado: %v", containerName, err)}
		}

		var protPorts map[int]bool
		proxyPorts, protPorts = extractProxyPortBindings(proxyInspect)
		protectedBindings = extractProtectedBindings(proxyInspect, protPorts)

		// Collect candidate new ports (not already open on running proxy)
		for _, sp := range specs {
			if !proxyPorts[sp.Port] || !proxyInspect.State.Running {
				newPorts = append(newPorts, sp.Port)
			}
		}

		if len(newPorts) > 0 {
			otherPorts := listOtherDockerHostPorts(client, proxyInspect.ID)
			gatewayIP := pickGatewayIP(proxyInspect)
			probeImage := selfContainerImage(client)
			if probeImage == "" {
				probeImage = proxyInspect.Image
			}

			for _, p := range newPorts {
				if owner, busy := otherPorts[p]; busy {
					return ApplyPortsResult{
						Mode:    mode,
						Message: fmt.Sprintf("Porta %d já está em uso pelo container Docker '%s'. Escolha outra porta.", p, owner),
					}
				}
				if gatewayIP != "" && canDialTCP(gatewayIP, p) {
					return ApplyPortsResult{
						Mode:    mode,
						Message: fmt.Sprintf("Porta %d já está em uso no servidor host (%s:%d respondeu).", p, gatewayIP, p),
					}
				}
				if busy, msg := probeHostPortViaDockerBind(client, probeImage, p); busy {
					return ApplyPortsResult{
						Mode:    mode,
						Message: fmt.Sprintf("Porta %d indisponível no host: %s", p, msg),
					}
				}
			}
		}
	}

	var prevComposeBytes []byte
	composeUpdated := false
	if wantCompose {
		svcName := composeService
		if svcName == "" && proxyInspect != nil {
			if labels, ok := proxyInspect.Config["Labels"].(map[string]any); ok {
				if svc, ok := labels["com.docker.compose.service"].(string); ok && svc != "" {
					svcName = svc
				}
			}
		}
		if svcName == "" {
			svcName = "email-oauth2-proxy"
		}

		origBytes, err := os.ReadFile(resolvedCompose)
		if err != nil {
			return ApplyPortsResult{Mode: mode, Message: fmt.Sprintf("Erro ao ler %s: %v", resolvedCompose, err)}
		}
		prevComposeBytes = origBytes

		updatedYAML, err := UpdateComposePortsYAML(string(origBytes), svcName, specs)
		if err != nil {
			return ApplyPortsResult{Mode: mode, Message: fmt.Sprintf("Erro ao atualizar YAML: %v", err)}
		}
		if err := WriteComposeFile(resolvedCompose, updatedYAML); err != nil {
			return ApplyPortsResult{Mode: mode, Message: fmt.Sprintf("Erro ao salvar %s: %v", resolvedCompose, err)}
		}
		composeUpdated = true
	}

	if wantDocker {
		recreated, rolledBack, err := recreateProxyWithPorts(client, proxyInspect, specs, protectedBindings)
		if err != nil {
			// Restore previous Compose file if Docker recreation failed and rolled back
			if composeUpdated && len(prevComposeBytes) > 0 {
				_ = WriteComposeFile(resolvedCompose, string(prevComposeBytes))
			}
			return ApplyPortsResult{
				OK:              false,
				Mode:            mode,
				DockerRecreated: recreated,
				ComposeUpdated:  false,
				ComposePath:     resolvedCompose,
				RolledBack:      rolledBack,
				Message:         err.Error(),
			}
		}
	}

	msg := "Portas atualizadas com sucesso!"
	switch {
	case wantCompose && wantDocker:
		msg = fmt.Sprintf("docker-compose.yml atualizado (%s) e container %s recriado com as novas portas!", resolvedCompose, containerName)
	case wantDocker:
		msg = fmt.Sprintf("Container %s recriado via Docker com as portas selecionadas! (Lembre-se de copiar o YAML se atualizar o app pelo Runtipi depois.)", containerName)
	case wantCompose:
		msg = fmt.Sprintf("Arquivo %s atualizado! Reinicie o app pelo Runtipi para aplicar.", resolvedCompose)
	}

	return ApplyPortsResult{
		OK:              true,
		Mode:            mode,
		DockerRecreated: wantDocker,
		ComposeUpdated:  composeUpdated,
		ComposePath:     resolvedCompose,
		Message:         msg,
	}
}

func recreateProxyWithPorts(
	client *http.Client,
	old *dockerContainerInspect,
	specs []PortSpec,
	protectedBindings map[string][]map[string]string,
) (recreated bool, rolledBack bool, err error) {
	origName := strings.TrimPrefix(old.Name, "/")
	if origName == "" {
		origName = "email-oauth2-proxy"
	}
	wasRunning := old.State.Running

	// Build updated ExposedPorts & PortBindings preserving protected bindings (e.g. 80/tcp -> 1999)
	// and preserving exact HostIp/IPv4/IPv6 bindings for ports already open on the proxy.
	existingBindings := extractRawPortBindings(old)
	exposed := make(map[string]struct{})
	bindings := make(map[string][]map[string]string)

	for portKey, bList := range protectedBindings {
		exposed[portKey] = struct{}{}
		bindings[portKey] = bList
	}
	for _, sp := range specs {
		key := fmt.Sprintf("%d/tcp", sp.Port)
		exposed[key] = struct{}{}
		if _, already := bindings[key]; !already {
			if prev, ok := existingBindings[key]; ok && len(prev) > 0 {
				bindings[key] = prev
			} else {
				bindings[key] = []map[string]string{
					{
						"HostIp":   "",
						"HostPort": strconv.Itoa(sp.Port),
					},
				}
			}
		}
	}

	newCfg := cloneJSONMap(old.Config)
	if old.ID != "" && len(old.ID) >= 12 {
		if hn, ok := newCfg["Hostname"].(string); ok && hn == old.ID[:12] {
			delete(newCfg, "Hostname")
		}
	}
	newCfg["ExposedPorts"] = exposed

	newHostCfg := cloneJSONMap(old.HostConfig)
	newHostCfg["PortBindings"] = bindings

	// Prepare NetworkingConfig for primary network, and extra networks to connect after create
	var primaryNetName string
	var extraNets []struct {
		name    string
		aliases []string
		ipam    map[string]any
		links   []string
		opts    map[string]any
	}
	endpointsCfg := make(map[string]any)

	netNames := make([]string, 0, len(old.NetworkSettings.Networks))
	for n := range old.NetworkSettings.Networks {
		netNames = append(netNames, n)
	}
	sort.Strings(netNames)

	shortID := ""
	if len(old.ID) >= 12 {
		shortID = old.ID[:12]
	}
	for idx, netName := range netNames {
		ns := old.NetworkSettings.Networks[netName]
		cleanAliases := make([]string, 0, len(ns.Aliases))
		for _, a := range ns.Aliases {
			if a != "" && a != shortID && a != old.ID {
				cleanAliases = append(cleanAliases, a)
			}
		}
		if idx == 0 {
			primaryNetName = netName
			ep := map[string]any{}
			if len(cleanAliases) > 0 {
				ep["Aliases"] = cleanAliases
			}
			if ns.IPAMConfig != nil {
				ep["IPAMConfig"] = ns.IPAMConfig
			}
			if len(ns.Links) > 0 {
				ep["Links"] = ns.Links
			}
			if ns.DriverOpts != nil {
				ep["DriverOpts"] = ns.DriverOpts
			}
			endpointsCfg[netName] = ep
		} else {
			extraNets = append(extraNets, struct {
				name    string
				aliases []string
				ipam    map[string]any
				links   []string
				opts    map[string]any
			}{
				name:    netName,
				aliases: cleanAliases,
				ipam:    ns.IPAMConfig,
				links:   ns.Links,
				opts:    ns.DriverOpts,
			})
		}
	}

	if primaryNetName != "" {
		newHostCfg["NetworkMode"] = primaryNetName
	}

	createBody := cloneJSONMap(newCfg)
	createBody["HostConfig"] = newHostCfg
	if len(endpointsCfg) > 0 {
		createBody["NetworkingConfig"] = map[string]any{
			"EndpointsConfig": endpointsCfg,
		}
	}

	// Step 1: Stop existing proxy container (do NOT delete yet)
	if wasRunning {
		if err := dockerPostNoBody(client, fmt.Sprintf("http://docker/containers/%s/stop?t=5", url.PathEscape(old.ID))); err != nil {
			return false, false, fmt.Errorf("falha ao parar container antigo: %w", err)
		}
	}

	// Step 2: Rename old container to a temporary backup name
	backupName := fmt.Sprintf("%s-bak-%d", origName, time.Now().Unix())
	if err := dockerPostNoBody(client, fmt.Sprintf("http://docker/containers/%s/rename?name=%s", url.PathEscape(old.ID), url.QueryEscape(backupName))); err != nil {
		if wasRunning {
			_ = dockerPostNoBody(client, fmt.Sprintf("http://docker/containers/%s/start", url.PathEscape(old.ID)))
		}
		return false, true, fmt.Errorf("falha ao renomear container para backup: %w", err)
	}

	rollbackOld := func(cause error) (bool, bool, error) {
		_ = dockerPostNoBody(client, fmt.Sprintf("http://docker/containers/%s/rename?name=%s", url.PathEscape(old.ID), url.QueryEscape(origName)))
		if wasRunning {
			_ = dockerPostNoBody(client, fmt.Sprintf("http://docker/containers/%s/start", url.PathEscape(old.ID)))
		}
		return false, true, fmt.Errorf("%w — container original '%s' foi restaurado sem perda", cause, origName)
	}

	// Step 3: Create new container under original name
	rawCreate, _ := json.Marshal(createBody)
	createReq, _ := http.NewRequest(http.MethodPost, "http://docker/containers/create?name="+url.QueryEscape(origName), bytes.NewReader(rawCreate))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := client.Do(createReq)
	if err != nil {
		return rollbackOld(fmt.Errorf("erro ao criar novo container: %w", err))
	}
	createRespBody, _ := io.ReadAll(io.LimitReader(createResp.Body, 8192))
	_ = createResp.Body.Close()
	if createResp.StatusCode >= 400 {
		return rollbackOld(fmt.Errorf("Docker recusou criar container (%d): %s", createResp.StatusCode, strings.TrimSpace(string(createRespBody))))
	}

	var created struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(createRespBody, &created); err != nil || created.ID == "" {
		return rollbackOld(fmt.Errorf("resposta inválida do Docker ao criar container"))
	}

	// Step 4: Connect any extra networks
	for _, en := range extraNets {
		epCfg := map[string]any{}
		if len(en.aliases) > 0 {
			epCfg["Aliases"] = en.aliases
		}
		if en.ipam != nil {
			epCfg["IPAMConfig"] = en.ipam
		}
		connPayload, _ := json.Marshal(map[string]any{
			"Container":      created.ID,
			"EndpointConfig": epCfg,
		})
		connReq, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://docker/networks/%s/connect", url.PathEscape(en.name)), bytes.NewReader(connPayload))
		connReq.Header.Set("Content-Type", "application/json")
		connResp, connErr := client.Do(connReq)
		if connResp != nil {
			_ = connResp.Body.Close()
		}
		if connErr != nil || (connResp != nil && connResp.StatusCode >= 400) {
			_ = dockerDelete(client, fmt.Sprintf("http://docker/containers/%s?force=true", url.PathEscape(created.ID)))
			return rollbackOld(fmt.Errorf("falha ao conectar rede '%s'", en.name))
		}
	}

	// Step 5: Start new container (this actually binds all host ports!)
	if err := dockerPostNoBody(client, fmt.Sprintf("http://docker/containers/%s/start", url.PathEscape(created.ID))); err != nil {
		_ = dockerDelete(client, fmt.Sprintf("http://docker/containers/%s?force=true", url.PathEscape(created.ID)))
		return rollbackOld(fmt.Errorf("falha ao iniciar com as novas portas: %w", err))
	}

	// Step 6: Remove backup container after new container started cleanly
	_ = dockerDelete(client, fmt.Sprintf("http://docker/containers/%s?force=true", url.PathEscape(old.ID)))
	return true, false, nil
}

func probeHostPortViaDockerBind(client *http.Client, image string, port int) (busy bool, msg string) {
	if image == "" {
		return false, ""
	}
	payload, _ := json.Marshal(map[string]any{
		"Image":      image,
		"Entrypoint": []string{"/bin/sh", "-c", "exit 0"},
		"Cmd":        []string{},
		"ExposedPorts": map[string]any{
			fmt.Sprintf("%d/tcp", port): struct{}{},
		},
		"HostConfig": map[string]any{
			"NetworkMode": "bridge",
			"PortBindings": map[string]any{
				fmt.Sprintf("%d/tcp", port): []map[string]string{
					{
						"HostIp":   "",
						"HostPort": strconv.Itoa(port),
					},
				},
			},
		},
	})

	req, err := http.NewRequest(http.MethodPost, "http://docker/containers/create", bytes.NewReader(payload))
	if err != nil {
		return false, ""
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return false, ""
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()
	if resp.StatusCode >= 400 {
		return false, ""
	}

	var created struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(body, &created); err != nil || created.ID == "" {
		return false, ""
	}
	defer func() {
		_ = dockerDelete(client, fmt.Sprintf("http://docker/containers/%s?force=true", url.PathEscape(created.ID)))
	}()

	if err := dockerPostNoBody(client, fmt.Sprintf("http://docker/containers/%s/start", url.PathEscape(created.ID))); err != nil {
		low := strings.ToLower(err.Error())
		if strings.Contains(low, "allocated") || strings.Contains(low, "in use") || strings.Contains(low, "bind") {
			return true, fmt.Sprintf("Porta %d ocupada no servidor host (detectado no teste de bind)", port)
		}
		return false, ""
	}
	return false, "Livre (testada com bind real no host)"
}

func extractRawPortBindings(ins *dockerContainerInspect) map[string][]map[string]string {
	out := make(map[string][]map[string]string)
	pb, ok := ins.HostConfig["PortBindings"].(map[string]any)
	if !ok {
		return out
	}
	for key, rawList := range pb {
		arr, ok := rawList.([]any)
		if !ok {
			continue
		}
		var mapped []map[string]string
		for _, item := range arr {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			hip, _ := m["HostIp"].(string)
			hp, _ := m["HostPort"].(string)
			mapped = append(mapped, map[string]string{
				"HostIp":   hip,
				"HostPort": hp,
			})
		}
		if len(mapped) > 0 {
			out[key] = mapped
		}
	}
	return out
}

func inspectContainer(client *http.Client, nameOrID string) (*dockerContainerInspect, error) {
	resp, err := client.Get(fmt.Sprintf("http://docker/containers/%s/json", url.PathEscape(nameOrID)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out dockerContainerInspect
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func inspectSelfPidShared(client *http.Client) bool {
	candidates := []string{}
	if hn, err := os.Hostname(); err == nil && hn != "" {
		candidates = append(candidates, hn)
	}
	candidates = append(candidates, "email-oauth2-proxy-ui")

	for _, c := range candidates {
		ins, err := inspectContainer(client, c)
		if err != nil {
			continue
		}
		if pm, ok := ins.HostConfig["PidMode"].(string); ok && strings.HasPrefix(pm, "container:") {
			return true
		}
		return false
	}
	return false
}

func selfContainerImage(client *http.Client) string {
	candidates := []string{}
	if hn, err := os.Hostname(); err == nil && hn != "" {
		candidates = append(candidates, hn)
	}
	candidates = append(candidates, "email-oauth2-proxy-ui")
	for _, c := range candidates {
		if ins, err := inspectContainer(client, c); err == nil && ins.Image != "" {
			return ins.Image
		}
	}
	return ""
}

func extractProxyPortBindings(ins *dockerContainerInspect) (published map[int]bool, protected map[int]bool) {
	published = make(map[int]bool)
	protected = make(map[int]bool)
	pb, ok := ins.HostConfig["PortBindings"].(map[string]any)
	if !ok {
		return published, protected
	}
	for key, rawList := range pb {
		parts := strings.Split(key, "/")
		if len(parts) != 2 || strings.ToLower(parts[1]) != "tcp" {
			continue
		}
		containerPort, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		arr, ok := rawList.([]any)
		if !ok {
			continue
		}
		for _, item := range arr {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			hpStr, _ := m["HostPort"].(string)
			hp, err := strconv.Atoi(strings.TrimSpace(hpStr))
			if err != nil || hp <= 0 {
				continue
			}
			published[hp] = true
			if containerPort == 80 {
				protected[hp] = true
			}
		}
	}
	return published, protected
}

func extractProtectedBindings(ins *dockerContainerInspect, protectedHostPorts map[int]bool) map[string][]map[string]string {
	out := make(map[string][]map[string]string)
	pb, ok := ins.HostConfig["PortBindings"].(map[string]any)
	if !ok {
		return out
	}
	for key, rawList := range pb {
		parts := strings.Split(key, "/")
		if len(parts) != 2 {
			continue
		}
		cPort, _ := strconv.Atoi(parts[0])
		isProtected := strings.ToLower(parts[1]) != "tcp" || cPort == 80
		arr, ok := rawList.([]any)
		if !ok {
			continue
		}
		var mapped []map[string]string
		for _, item := range arr {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			hip, _ := m["HostIp"].(string)
			hp, _ := m["HostPort"].(string)
			if hpInt, err := strconv.Atoi(strings.TrimSpace(hp)); err == nil && protectedHostPorts[hpInt] {
				isProtected = true
			}
			mapped = append(mapped, map[string]string{
				"HostIp":   hip,
				"HostPort": hp,
			})
		}
		if isProtected && len(mapped) > 0 {
			out[key] = mapped
		}
	}
	return out
}

func listOtherDockerHostPorts(client *http.Client, skipContainerID string) map[int]string {
	out := make(map[int]string)
	resp, err := client.Get("http://docker/containers/json?all=1")
	if err != nil {
		return out
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if resp.StatusCode >= 400 {
		return out
	}
	var list []dockerContainerSummary
	if err := json.Unmarshal(body, &list); err != nil {
		return out
	}
	for _, c := range list {
		if c.ID == skipContainerID {
			continue
		}
		name := c.ID
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		for _, p := range c.Ports {
			if p.PublicPort > 0 && strings.EqualFold(p.Type, "tcp") {
				if _, exists := out[p.PublicPort]; !exists {
					out[p.PublicPort] = name
				}
			}
		}
	}
	return out
}

func pickGatewayIP(ins *dockerContainerInspect) string {
	for _, n := range ins.NetworkSettings.Networks {
		if n.Gateway != "" {
			return n.Gateway
		}
	}
	return ""
}

func dockerPostNoBody(client *http.Client, endpoint string) error {
	req, err := http.NewRequest(http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func dockerDelete(client *http.Client, endpoint string) error {
	req, err := http.NewRequest(http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func cloneJSONMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	raw, _ := json.Marshal(src)
	var dst map[string]any
	_ = json.Unmarshal(raw, &dst)
	if dst == nil {
		return map[string]any{}
	}
	return dst
}

func sortedKeysInt(m map[int]bool) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

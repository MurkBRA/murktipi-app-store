package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"email-oauth2-proxy-ui/internal/server"
)

//go:embed web/*
var embeddedWeb embed.FS

func main() {
	listenAddr := envOr("LISTEN_ADDR", ":8090")
	configDir := envOr("CONFIG_DIR", "/config")
	proxyHost := envOr("PROXY_HOST", "email-oauth2-proxy")
	proxyContainer := envOr("PROXY_CONTAINER", proxyHost)
	dockerSocket := envOr("DOCKER_SOCKET", "/var/run/docker.sock")
	composePath := envOr("COMPOSE_FILE", envOr("COMPOSE_DIR", "/compose"))
	composeService := envOr("COMPOSE_SERVICE", "email-oauth2-proxy")
	adminUser := envOr("ADMIN_USER", "admin")
	adminPass := os.Getenv("ADMIN_PASSWORD")

	webFS, err := fs.Sub(embeddedWeb, "web")
	if err != nil {
		log.Fatalf("embed web fs: %v", err)
	}

	srv := server.New(server.Options{
		ConfigDir:      configDir,
		ConfigFile:     os.Getenv("CONFIG_FILE"),
		CredStoreFile:  os.Getenv("CREDSTORE_FILE"),
		LogFile:        os.Getenv("LOG_FILE"),
		ProxyHost:      proxyHost,
		ProxyContainer: proxyContainer,
		DockerSocket:   dockerSocket,
		ComposePath:    composePath,
		ComposeService: composeService,
		AdminUser:      adminUser,
		AdminPassword:  adminPass,
		WebFS:          webFS,
	})

	monitorCtx, cancelMonitor := context.WithCancel(context.Background())
	defer cancelMonitor()
	go srv.StartMonitor(monitorCtx)

	httpServer := &http.Server{
		Addr:              listenAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("Email OAuth2 Proxy WebGUI listening on %s (config=%s, proxy=%s, auth=%v)",
			listenAddr, configDir, proxyHost, adminPass != "")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down gracefully...")
	cancelMonitor()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

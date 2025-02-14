// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpd // import "miniflux.app/v2/http/server"

import (
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"miniflux.app/v2/api"
	"miniflux.app/v2/config"
	"miniflux.app/v2/fever"
	"miniflux.app/v2/googlereader"
	"miniflux.app/v2/http/request"
	"miniflux.app/v2/logger"
	"miniflux.app/v2/storage"
	"miniflux.app/v2/ui"
	"miniflux.app/v2/version"
	"miniflux.app/v2/worker"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

func StartWebServer(store *storage.Storage, pool *worker.Pool) *http.Server {
	certFile := config.Opts.CertFile()
	keyFile := config.Opts.CertKeyFile()
	certDomain := config.Opts.CertDomain()
	listenAddr := config.Opts.ListenAddr()
	server := &http.Server{
		ReadTimeout:  time.Duration(config.Opts.HTTPServerTimeout()) * time.Second,
		WriteTimeout: time.Duration(config.Opts.HTTPServerTimeout()) * time.Second,
		IdleTimeout:  time.Duration(config.Opts.HTTPServerTimeout()) * time.Second,
		Handler:      setupHandler(store, pool),
	}

	switch {
	case os.Getenv("LISTEN_PID") == strconv.Itoa(os.Getpid()):
		startSystemdSocketServer(server)
	case strings.HasPrefix(listenAddr, "/"):
		startUnixSocketServer(server, listenAddr)
	case certDomain != "":
		config.Opts.HTTPS = true
		startAutoCertTLSServer(server, certDomain, store)
	case certFile != "" && keyFile != "":
		config.Opts.HTTPS = true
		server.Addr = listenAddr
		startTLSServer(server, certFile, keyFile)
	default:
		server.Addr = listenAddr
		startHTTPServer(server)
	}

	return server
}

func startSystemdSocketServer(server *http.Server) {
	go func() {
		f := os.NewFile(3, "systemd socket")
		listener, err := net.FileListener(f)
		if err != nil {
			logger.Fatal(`Unable to create listener from systemd socket: %v`, err)
		}

		logger.Info(`Listening on systemd socket`)
		if err := server.Serve(listener); err != http.ErrServerClosed {
			logger.Fatal(`Server failed to start: %v`, err)
		}
	}()
}

func startUnixSocketServer(server *http.Server, socketFile string) {
	os.Remove(socketFile)

	go func(sock string) {
		listener, err := net.Listen("unix", sock)
		if err != nil {
			logger.Fatal(`Server failed to start: %v`, err)
		}
		defer listener.Close()

		if err := os.Chmod(sock, 0666); err != nil {
			logger.Fatal(`Unable to change socket permission: %v`, err)
		}

		logger.Info(`Listening on Unix socket %q`, sock)
		if err := server.Serve(listener); err != http.ErrServerClosed {
			logger.Fatal(`Server failed to start: %v`, err)
		}
	}(socketFile)
}

func tlsConfig() *tls.Config {
	// See https://blog.cloudflare.com/exposing-go-on-the-internet/
	// And https://wikia.mozilla.org/Security/Server_Side_TLS
	return &tls.Config{
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
		CurvePreferences: []tls.CurveID{
			tls.CurveP256,
			tls.X25519,
		},
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}
}

func startAutoCertTLSServer(server *http.Server, certDomain string, store *storage.Storage) {
	server.Addr = ":https"
	certManager := autocert.Manager{
		Cache:      storage.NewCertificateCache(store),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(certDomain),
	}
	server.TLSConfig = tlsConfig()
	server.TLSConfig.GetCertificate = certManager.GetCertificate
	server.TLSConfig.NextProtos = []string{"h2", "http/1.1", acme.ALPNProto}

	// Handle http-01 challenge.
	s := &http.Server{
		Handler: certManager.HTTPHandler(nil),
		Addr:    ":http",
	}
	go s.ListenAndServe()

	go func() {
		logger.Info(`Listening on %q by using auto-configured certificate for %q`, server.Addr, certDomain)
		if err := server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
			logger.Fatal(`Server failed to start: %v`, err)
		}
	}()
}

func startTLSServer(server *http.Server, certFile, keyFile string) {
	server.TLSConfig = tlsConfig()
	go func() {
		logger.Info(`Listening on %q by using certificate %q and key %q`, server.Addr, certFile, keyFile)
		if err := server.ListenAndServeTLS(certFile, keyFile); err != http.ErrServerClosed {
			logger.Fatal(`Server failed to start: %v`, err)
		}
	}()
}

func startHTTPServer(server *http.Server) {
	go func() {
		logger.Info(`Listening on %q without TLS`, server.Addr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			logger.Fatal(`Server failed to start: %v`, err)
		}
	}()
}

func setupHandler(store *storage.Storage, pool *worker.Pool) *mux.Router {
	router := mux.NewRouter()

	if config.Opts.BasePath() != "" {
		router = router.PathPrefix(config.Opts.BasePath()).Subrouter()
	}

	if config.Opts.HasMaintenanceMode() {
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(config.Opts.MaintenanceMessage()))
			})
		})
	}

	router.Use(middleware)

	fever.Serve(router, store)
	googlereader.Serve(router, store)
	api.Serve(router, store, pool)
	ui.Serve(router, store, pool)

	router.HandleFunc("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		if err := store.Ping(); err != nil {
			http.Error(w, "Database Connection Error", http.StatusInternalServerError)
			return
		}

		w.Write([]byte("OK"))
	}).Name("healthcheck")

	router.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(version.Version))
	}).Name("version")

	if config.Opts.HasMetricsCollector() {
		router.Handle("/metrics", promhttp.Handler()).Name("metrics")
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				route := mux.CurrentRoute(r)

				// Returns a 404 if the client is not authorized to access the metrics endpoint.
				if route.GetName() == "metrics" && !isAllowedToAccessMetricsEndpoint(r) {
					logger.Error(`[Metrics] [ClientIP=%s] Client not allowed (%s)`, request.ClientIP(r), r.RemoteAddr)
					http.NotFound(w, r)
					return
				}

				next.ServeHTTP(w, r)
			})
		})
	}

	return router
}

func isAllowedToAccessMetricsEndpoint(r *http.Request) bool {
	if config.Opts.MetricsUsername() != "" && config.Opts.MetricsPassword() != "" {
		clientIP := request.ClientIP(r)
		username, password, authOK := r.BasicAuth()
		if !authOK {
			logger.Info("[Metrics] [ClientIP=%s] No authentication header sent", clientIP)
			return false
		}

		if username == "" || password == "" {
			logger.Info("[Metrics] [ClientIP=%s] Empty username or password", clientIP)
			return false
		}

		if username != config.Opts.MetricsUsername() || password != config.Opts.MetricsPassword() {
			logger.Error("[Metrics] [ClientIP=%s] Invalid username or password", clientIP)
			return false
		}
	}

	for _, cidr := range config.Opts.MetricsAllowedNetworks() {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			logger.Fatal(`[Metrics] Unable to parse CIDR %v`, err)
		}

		// We use r.RemoteAddr in this case because HTTP headers like X-Forwarded-For can be easily spoofed.
		// The recommendation is to use HTTP Basic authentication.
		if network.Contains(net.ParseIP(request.FindRemoteIP(r))) {
			return true
		}
	}

	return false
}

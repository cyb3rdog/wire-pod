package initwirepod

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"

	chipperpb "github.com/digital-dream-labs/api/go/chipperpb"
	"github.com/digital-dream-labs/api/go/jdocspb"
	"github.com/digital-dream-labs/api/go/tokenpb"
	"github.com/digital-dream-labs/hugh/log"
	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	"github.com/kercre123/wire-pod/chipper/pkg/mdnshandler"
	chipperserver "github.com/kercre123/wire-pod/chipper/pkg/servers/chipper"
	jdocsserver "github.com/kercre123/wire-pod/chipper/pkg/servers/jdocs"
	tokenserver "github.com/kercre123/wire-pod/chipper/pkg/servers/token"
	"github.com/kercre123/wire-pod/chipper/pkg/vars"
	wpweb "github.com/kercre123/wire-pod/chipper/pkg/wirepod/config-ws"
	wp "github.com/kercre123/wire-pod/chipper/pkg/wirepod/preqs"
	sdkWeb "github.com/kercre123/wire-pod/chipper/pkg/wirepod/sdkapp"
	botsetup "github.com/kercre123/wire-pod/chipper/pkg/wirepod/setup"
	"github.com/soheilhy/cmux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	//	grpclog "github.com/digital-dream-labs/hugh/grpc/interceptors/logger"

	grpcserver "github.com/digital-dream-labs/hugh/grpc/server"
)

var PostingmDNS bool

var serverOne cmux.CMux
var serverTwo cmux.CMux
var listenerOne net.Listener
var listenerTwo net.Listener
var voiceProcessor *wp.Server

// grpcServer *grpc.Servervar
var chipperServing bool = false

func serveOk(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "ok")
}

func httpServe(l net.Listener) error {
	mux := http.NewServeMux()

	// Security headers middleware
	securityHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		mux.ServeHTTP(w, r)
	})

	mux.HandleFunc("/ok:80", serveOk)
	mux.HandleFunc("/ok", serveOk)
	s := &http.Server{
		Handler: securityHandler,
	}
	return s.Serve(l)
}

// recoverUnary and recoverStream stop a panic in one RPC handler (e.g. from
// malformed robot-supplied input) from crashing the process for every
// connected robot. gRPC-go does not recover handler panics on its own.
func recoverUnary(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			logger.Println("grpc: recovered panic in", info.FullMethod, ":", rec, "\n", string(debug.Stack()))
			err = status.Errorf(codes.Internal, "internal error")
		}
	}()
	return handler(ctx, req)
}

func recoverStream(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			logger.Println("grpc: recovered panic in", info.FullMethod, ":", rec, "\n", string(debug.Stack()))
			err = status.Errorf(codes.Internal, "internal error")
		}
	}()
	return handler(srv, ss)
}

func grpcServe(l net.Listener, p *wp.Server) error {
	srv, err := grpcserver.New(
		grpcserver.WithViper(),
		grpcserver.WithReflectionService(),
		grpcserver.WithInsecureSkipVerify(),
		grpcserver.WithUnaryServerInterceptors(recoverUnary),
		grpcserver.WithStreamServerInterceptors(recoverStream),
	)
	if err != nil {
		log.Fatal(err)
	}

	s, _ := chipperserver.New(
		chipperserver.WithIntentProcessor(p),
		chipperserver.WithKnowledgeGraphProcessor(p),
		chipperserver.WithIntentGraphProcessor(p),
	)

	tokenServer := tokenserver.NewTokenServer()
	jdocsServer := jdocsserver.NewJdocsServer()
	//jdocsserver.IniToJson()

	chipperpb.RegisterChipperGrpcServer(srv.Transport(), s)
	jdocspb.RegisterJdocsServer(srv.Transport(), jdocsServer)
	tokenpb.RegisterTokenServer(srv.Transport(), tokenServer)

	return srv.Transport().Serve(l)
}

func BeginWirepodSpecific(sttInitFunc func() error, sttHandlerFunc interface{}, voiceProcessorName string) error {
	logger.Init()

	// begin wirepod stuff
	vars.Init()
	ensureCertForHostOverride()
	var err error
	voiceProcessor, err = wp.New(sttInitFunc, sttHandlerFunc, voiceProcessorName)
	// BeginServer always binds :80 -- even with SDK_ENABLED=false, it
	// still needs to serve /ok, which server_config.json's "check" field
	// points paired robots at for their basic connectivity check-in.
	// SDK_ENABLED only decides what else BeginServer registers alongside
	// that. See sdkapp.BeginServer.
	go sdkWeb.BeginServer()
	http.HandleFunc("/api-chipper/", ChipperHTTPApi)
	if err != nil {
		return err
	}
	return nil
}

// ensureCertForHostOverride generates the TLS cert/server_config.json for
// a HOST_OVERRIDE-seeded deployment (vars.CreateConfigFromEnv) before the
// first StartChipper call, so that call already has the right cert --
// instead of requiring a trip through initial.html's connection-method
// form, which reconfigures an already-started server. That code
// (botsetup) imports vars, so vars can't call it directly and do this
// itself inside CreateConfigFromEnv.
//
// Only fires when no cert exists yet: on every later boot, the persisted
// config (and its cert) already reflect whatever's current, whether
// that's still this same HOST_OVERRIDE or a domain since changed via
// initial.html/the dashboard -- env vars only ever seed a genuinely fresh
// setup, same as every other setting in this codebase.
func ensureCertForHostOverride() {
	if vars.APIConfig.Server.HostOverride == "" {
		return
	}
	if _, err := os.Stat(vars.CertPath); err == nil {
		return
	}
	logger.Println("HOST_OVERRIDE set with no cert on disk yet -- generating one for " + vars.APIConfig.Server.HostOverride)
	if err := botsetup.CreateCertCombo(); err != nil {
		logger.Println("failed to generate cert for HOST_OVERRIDE:", err)
		return
	}
	botsetup.CreateServerConfig()
	vars.WriteConfigToDisk()
}

func StartFromProgramInit(sttInitFunc func() error, sttHandlerFunc interface{}, voiceProcessorName string) {
	if runtime.GOOS == "android" || runtime.GOOS == "ios" {
		os.Setenv("DEBUG_LOGGING", "true")
		os.Setenv("STT_SERVICE", "vosk")
	}
	err := BeginWirepodSpecific(sttInitFunc, sttHandlerFunc, voiceProcessorName)
	if err != nil {
		logger.Println("\033[33m\033[1mWire-pod is not setup. Use the webserver at port 8080 to set up wire-pod.\033[0m")
	} else if !vars.APIConfig.PastInitialSetup {
		logger.Println("\033[33m\033[1mWire-pod is not setup. Use the webserver at port 8080 to set up wire-pod.\033[0m")
	} else if (vars.APIConfig.STT.Service == "vosk" || vars.APIConfig.STT.Service == "whisper.cpp") && vars.APIConfig.STT.Language == "" {
		logger.Println("\033[33m\033[1mLanguage value is blank, but STT service is " + vars.APIConfig.STT.Service + ". Reinitiating setup process.\033[0m")
		logger.Println("\033[33m\033[1mWire-pod is not setup. Use the webserver at port 8080 to set up wire-pod.\033[0m")
		vars.APIConfig.PastInitialSetup = false
	} else {
		go StartChipper()
	}
	// main thread is configuration ws
	wpweb.StartWebServer()
}

// closeServers shuts down whichever of the two cmux/listener pairs are
// actually in use. serverTwo/listenerTwo are only assigned in StartChipper
// when the current config runs the legacy :8084 listener (EPConfig plus
// Port8084Enabled) -- IP mode and Custom Host mode never touch them, so
// they stay at their nil zero value. serverOne/listenerOne is nil-checked
// too for the same reason on Android (see StartChipper's early-return
// branch for port 443). Calling .Close() on a nil interface value panics
// unconditionally in Go, and RestartServer runs synchronously inside the
// /api-chipper/* HTTP handler -- so without this guard, reconfiguring an
// already-running non-EPConfig server (exactly what the setup page's
// connection-method form now also does, not just first-run) panicked
// every time, surfacing as "internal error" to the browser.
func closeServers() {
	if serverOne != nil {
		serverOne.Close()
	}
	if serverTwo != nil {
		serverTwo.Close()
	}
	if listenerOne != nil {
		listenerOne.Close()
	}
	if listenerTwo != nil {
		listenerTwo.Close()
	}
}

func RestartServer() {
	if chipperServing {
		closeServers()
	}
	go StartChipper()
}

func StopServer() {
	if chipperServing {
		closeServers()
	}
}

func StartChipper() {
	// load certs
	if vars.APIConfig.Server.EPConfig && runtime.GOOS != "android" {
		go mdnshandler.PostmDNS()
	}
	var certPub []byte
	var certPriv []byte
	if runtime.GOOS == "android" || runtime.GOOS == "ios" {
		if vars.APIConfig.Server.EPConfig {
			certPub, _ = os.ReadFile(vars.AndroidPath + "/static/epod/ep.crt")
			certPriv, _ = os.ReadFile(vars.AndroidPath + "/static/epod/ep.key")
		} else {
			var err error
			certPub, _ = os.ReadFile(vars.AndroidPath + "/wire-pod/certs/cert.crt")
			certPriv, err = os.ReadFile(vars.AndroidPath + "/wire-pod/certs/cert.key")
			if err != nil {
				logger.Println("wire-pod is not setup.")
				return
			}
		}
	} else {
		if vars.APIConfig.Server.EPConfig {
			certPub, _ = os.ReadFile("./epod/ep.crt")
			certPriv, _ = os.ReadFile("./epod/ep.key")
		} else {
			var err error
			certPub, _ = os.ReadFile("../certs/cert.crt")
			certPriv, err = os.ReadFile("../certs/cert.key")
			if err != nil {
				logger.Println("wire-pod is not setup.")
				return
			}
		}
	}

	logger.Println("Initiating TLS listener, cmux, gRPC handler, and REST handler")
	cert, err := tls.X509KeyPair(certPub, certPriv)
	if err != nil {
		logger.Println(err)
		os.Exit(1)
	}
	if runtime.GOOS == "android" && vars.APIConfig.Server.Port == "443" {
		logger.Println("not starting chipper at port 443 because android")
	} else {
		logger.Println("Starting chipper server at port " + vars.APIConfig.Server.Port)
		listenerOne, err = tls.Listen("tcp", ":"+vars.APIConfig.Server.Port, &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
		})
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
	serverOne = cmux.New(listenerOne)
	grpcListenerOne := serverOne.Match(cmux.HTTP2())
	httpListenerOne := serverOne.Match(cmux.HTTP1Fast())
	go grpcServe(grpcListenerOne, voiceProcessor)
	go httpServe(httpListenerOne)

	if vars.APIConfig.Server.EPConfig && vars.Port8084Enabled() {
		logger.Println("Starting chipper server at port 8084 for 2.0.1 compatibility")
		listenerTwo, err = tls.Listen("tcp", ":8084", &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
		})
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		serverTwo = cmux.New(listenerTwo)
		grpcListenerTwo := serverTwo.Match(cmux.HTTP2())
		httpListenerTwo := serverTwo.Match(cmux.HTTP1Fast())
		go grpcServe(grpcListenerTwo, voiceProcessor)
		go httpServe(httpListenerTwo)
	} else {
		// Not running this cycle -- clear out whatever a *previous* run
		// left behind (already closed by closeServers, but still
		// non-nil) so state accurately reflects "not in use" rather than
		// holding a stale, defunct reference indefinitely.
		serverTwo = nil
		listenerTwo = nil
	}

	fmt.Println("\033[33m\033[1mwire-pod started successfully!\033[0m")

	chipperServing = true
	if vars.APIConfig.Server.EPConfig && vars.Port8084Enabled() {
		if runtime.GOOS != "android" {
			go serverOne.Serve()
		}
		serverTwo.Serve()
		logger.Println("Stopping chipper server")
		chipperServing = false
	} else {
		serverOne.Serve()
		logger.Println("Stopping chipper server")
		chipperServing = false
	}
}

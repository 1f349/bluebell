package main

import (
	"context"
	"errors"
	"flag"
	"github.com/1f349/bluebell/conf"
	"github.com/1f349/bluebell/logger"
	"github.com/1f349/bluebell/serve"
	"github.com/1f349/bluebell/upload"
	"github.com/charmbracelet/log"
	"github.com/cloudflare/tableflip"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

var (
	configPath = flag.String("conf", "", "Config file path")
	debugLog   = flag.Bool("debug", false, "Enable debug logging")
	pidFile    = flag.String("pid-file", "", "Path to pid file")
)

func main() {
	flag.Parse()
	if *debugLog {
		logger.Logger.SetLevel(log.DebugLevel)
	}
	logger.Logger.Info("Starting...")

	upg, err := tableflip.New(tableflip.Options{
		PIDFile: *pidFile,
	})
	if err != nil {
		panic(err)
	}
	defer upg.Stop()

	if *configPath == "" {
		logger.Logger.Error("Config flag is missing")
		os.Exit(1)
	}

	openConf, err := os.Open(*configPath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Logger.Error("Missing config file")
		} else {
			logger.Logger.Error("Open config file", "err", err)
		}
		os.Exit(1)
	}

	var config conf.Conf
	err = yaml.NewDecoder(openConf).Decode(&config)
	if err != nil {
		logger.Logger.Error("Invalid config file", "err", err)
		os.Exit(1)
	}

	wd := filepath.Dir(*configPath)
	sitesDir := filepath.Join(wd, "sites")

	_, err = os.Stat(sitesDir)
	if err != nil {
		logger.Logger.Fatal("Failed to find sites, does the directory exist? Error: ", err)
	}

	sitesFs := afero.NewBasePathFs(afero.NewOsFs(), sitesDir)

	// Do an upgrade on SIGHUP
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGHUP)
		for range sig {
			err := upg.Upgrade()
			if err != nil {
				logger.Logger.Error("Failed upgrade", "err", err)
			}
		}
	}()

	// Listen must be called before Ready
	ln, err := upg.Listen("tcp", config.Listen)
	if err != nil {
		logger.Logger.Fatal("Listen failed", "err", err)
	}

	uploadHandler := upload.New(sitesFs)
	serveHandler := serve.New(sitesFs)

	router := httprouter.New()
	router.POST("/u/:site", uploadHandler.Handle)
	router.GET("/*filepath", serveHandler.Handle)

	server := &http.Server{
		Handler:           router,
		ReadTimeout:       1 * time.Minute,
		ReadHeaderTimeout: 1 * time.Minute,
		WriteTimeout:      1 * time.Minute,
		IdleTimeout:       1 * time.Minute,
		MaxHeaderBytes:    4_096_000,
	}
	logger.Logger.Info("HTTP server listening on", "addr", config.Listen)
	go func() {
		err := server.Serve(ln)
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Logger.Fatal("Serve failed", "err", err)
		}
	}()

	logger.Logger.Info("Ready")
	if err := upg.Ready(); err != nil {
		panic(err)
	}
	<-upg.Exit()

	time.AfterFunc(30*time.Second, func() {
		logger.Logger.Warn("Graceful shutdown timed out")
		os.Exit(1)
	})

	server.Shutdown(context.Background())
}

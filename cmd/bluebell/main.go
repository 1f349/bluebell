package main

import (
	"context"
	"errors"
	"flag"
	"github.com/1f349/bluebell"
	"github.com/1f349/bluebell/api"
	"github.com/1f349/bluebell/conf"
	"github.com/1f349/bluebell/hook"
	"github.com/1f349/bluebell/logger"
	"github.com/1f349/bluebell/peers"
	"github.com/1f349/bluebell/serve"
	"github.com/1f349/bluebell/upload"
	"github.com/1f349/mjwt"
	"github.com/charmbracelet/log"
	"github.com/cloudflare/tableflip"
	"github.com/dustin/go-humanize"
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
	uploadsDir := filepath.Join(wd, "uploads")
	sitesPostHookDir := filepath.Join(wd, "hooks/post")
	peersDir := filepath.Join(wd, "peers")

	keyStore, err := mjwt.NewKeyStoreFromPath(filepath.Join(wd, "keystore"))
	if err != nil {
		logger.Logger.Fatal("Failed to load MJWT keystore", "dir", filepath.Join(wd, "keystore"), "err", err)
	}

	err = os.MkdirAll(sitesDir, 0770)
	if err != nil {
		logger.Logger.Fatal("Failed to find or create sites directory", "err", err)
	}

	err = os.MkdirAll(sitesPostHookDir, 0770)
	if err != nil {
		logger.Logger.Fatal("Failed to find or create sites directory", "err", err)
	}

	err = os.MkdirAll(uploadsDir, 0770)
	if err != nil {
		logger.Logger.Fatal("Failed to find or create uploads directory", "err", err)
	}

	err = os.MkdirAll(uploadsDir, 0770)
	if err != nil {
		logger.Logger.Fatal("Failed to find or create uploads directory", "err", err)
	}

	err = os.MkdirAll(peersDir, 0770)
	if err != nil {
		logger.Logger.Fatal("Failed to find or create peers directory", "err", err)
	}

	sitesFs := afero.NewBasePathFs(afero.NewOsFs(), sitesDir)
	uploadsFs := afero.NewBasePathFs(afero.NewOsFs(), uploadsDir)
	peersFs := afero.NewBasePathFs(afero.NewOsFs(), peersDir)

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

	db, err := bluebell.InitDB(filepath.Join(wd, config.DB))
	if err != nil {
		logger.Logger.Fatal("Failed to open database", "err", err)
		return
	}

	// Listen must be called before Ready
	lnHttp, err := upg.Listen("tcp", config.Listen.Http)
	if err != nil {
		logger.Logger.Fatal("Listen failed", "err", err)
	}

	lnApi, err := upg.Listen("tcp", config.Listen.Api)
	if err != nil {
		logger.Logger.Fatal("Listen failed", "err", err)
	}

	peerManager, err := peers.New(peersFs)
	if err != nil {
		logger.Logger.Fatal("Failed to load peer manager", "err", err)
	}

	serveHandler := serve.New(sitesFs, db)
	postHook := hook.New(sitesPostHookDir, sitesDir)
	uploadHandler := upload.New(sitesFs, uploadsFs, db, postHook, peerManager)
	apiHandler := api.New(uploadHandler, keyStore, db)

	serverHttp := &http.Server{
		Handler:           serveHandler,
		ReadTimeout:       1 * time.Minute,
		ReadHeaderTimeout: 1 * time.Minute,
		WriteTimeout:      1 * time.Minute,
		IdleTimeout:       1 * time.Minute,
		MaxHeaderBytes:    4 * humanize.MiByte,
	}
	logger.Logger.Info("HTTP server listening on", "addr", config.Listen.Http)
	go func() {
		err := serverHttp.Serve(lnHttp)
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Logger.Fatal("Serve failed", "err", err)
		}
	}()

	serverApi := &http.Server{
		Handler:           apiHandler,
		ReadTimeout:       1 * time.Minute,
		ReadHeaderTimeout: 1 * time.Minute,
		WriteTimeout:      1 * time.Minute,
		IdleTimeout:       1 * time.Minute,
		MaxHeaderBytes:    4 * humanize.MiByte,
	}
	logger.Logger.Info("API server listening on", "addr", config.Listen.Api)
	go func() {
		err := serverApi.Serve(lnApi)
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Logger.Fatal("API Serve failed", "err", err)
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

	serverHttp.Shutdown(context.Background())
}

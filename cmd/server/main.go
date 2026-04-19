package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/livekit/livekit-server/pkg/config"
	"github.com/livekit/livekit-server/pkg/logger"
	"github.com/livekit/livekit-server/pkg/server"
	"github.com/livekit/protocol/livekit"
)

var version = "dev"

func main() {
	rand.Seed(time.Now().UnixNano())

	app := &cli.App{
		Name:    "livekit-server",
		Usage:   "LiveKit SFU server",
		Version: version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Usage:   "path to config file",
				EnvVars: []string{"LIVEKIT_CONFIG_FILE"},
			},
			&cli.StringFlag{
				Name:    "config-body",
				Usage:   "config body in YAML, can be used in place of a config file",
				EnvVars: []string{"LIVEKIT_CONFIG_BODY"},
			},
			&cli.StringFlag{
				Name:    "node-ip",
				Usage:   "IP address of the node, used to advertise to clients",
				EnvVars: []string{"NODE_IP"},
			},
			&cli.StringFlag{
				Name:    "redis",
				Usage:   "Redis URL, used for distributed deployments",
				EnvVars: []string{"REDIS_URL"},
			},
			&cli.StringFlag{
				Name:    "bind",
				Usage:   "address to bind to",
				EnvVars: []string{"LIVEKIT_BIND"},
			},
		},
		Action: startServer,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func startServer(c *cli.Context) error {
	conf, err := config.NewConfig(c.String("config"), c.String("config-body"), c)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	logger.InitFromConfig(&conf.Logging, "livekit")

	logger.Infow("starting LiveKit server",
		"version", version,
		"nodeID", livekit.NodeID(conf.NodeID),
		"portHTTP", conf.Port,
	)

	srv, err := server.InitializeServer(conf)
	if err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	sigChan := make(chan os.Signal, 1)
	// Also handle SIGHUP so the process can be gracefully stopped by init systems
	// Note: SIGUSR1 would be handy for log rotation but not adding that complexity here
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP)

	go func() {
		sig := <-sigChan
		logger.Infow("received signal, shutting down", "signal", sig)
		// graceful=true ensures in-progress sessions are allowed to finish
		srv.Stop(true)
	}()

	return srv.Start()
}

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
	// Handle common termination signals. SIGUSR1 is also trapped here to make it
	// easy to trigger a clean shutdown from scripts during local development
	// (e.g. `kill -USR1 <pid>` instead of SIGINT which can be noisy in tmux).
	// Note: SIGUSR2 is intentionally omitted; I use it externally for profiling.
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP, syscall.SIGUSR1)

	go func() {
		sig := <-sigChan
		logger.Infow("received signal, shutting down", "signal", sig)
		// Use graceful shutdown so in-flight requests can finish cleanly.
		// Previously set to false for quick local dev restarts, but graceful=true
		// is safer even locally to avoid torn connections during testing.
		srv.Stop(true)
	}()

	// Print a local-friendly startup banner so I can quickly confirm the process
	// is running when tailing logs without grepping.
	// Also log to stderr so it shows up even when stdout is redirected to a file.
	// Added start time so I can tell restarts apart at a glance.
	startTime := time.Now()
	fmt.Fprintf(os.Stderr, "[livekit-server %s] listening on port %d (started %s)\n",
		version, conf.Port, startTime.Format(time.RFC3339))

	return srv.Start()
}

// noports-agent is an SR Linux NDK application that makes NoPorts a native
// router feature: configuration comes from the SR Linux config tree
// (CLI/gNMI/JSON-RPC) and the agent supervises the sshnpd daemon, publishing
// operational state back into the state tree.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/srl-labs/bond"

	"github.com/atsign-foundation/noports-srlinux/agent/app"
)

var version = "0.0.0-dev"

func main() {
	versionFlag := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	// app_mgr captures stderr into /var/log/srlinux/stdout/<app>.log
	logger := zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		NoColor:    true,
		TimeFormat: "2006-01-02 15:04:05 MST",
	}).With().Timestamp().Logger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agent, errs := bond.NewAgent(app.AppName,
		bond.WithLogger(&logger),
		bond.WithContext(ctx, cancel),
		bond.WithAppRootPath(app.AppRoot),
	)
	for _, err := range errs {
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to create NDK agent")
		}
	}

	if err := agent.Start(); err != nil {
		logger.Fatal().Err(err).Msg("failed to start NDK agent")
	}

	a := app.New(&logger, agent, version)
	a.Start(ctx)
}

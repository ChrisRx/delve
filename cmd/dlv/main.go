package main

import (
	"os"

	"github.com/ChrisRx/delve/cmd/dlv/cmds"
	"github.com/ChrisRx/delve/pkg/logflags"
	"github.com/ChrisRx/delve/pkg/version"
	"golang.org/x/telemetry"
)

// Build is the git sha of this binaries build.
var Build string

func main() {
	telemetry.Start(telemetry.Config{
		ReportCrashes: true,
	})

	if Build != "" {
		version.DelveVersion.Build = Build
	}

	const cgoCflagsEnv = "CGO_CFLAGS"
	if os.Getenv(cgoCflagsEnv) == "" {
		os.Setenv(cgoCflagsEnv, "-O0 -g")
	} else {
		logflags.WriteCgoFlagsWarning()
	}

	cmds.New(false).Execute()
}

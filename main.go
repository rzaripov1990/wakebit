// Wakebit keeps the computer awake from the system tray.
package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"wakebit/internal/config"
	"wakebit/internal/engine"
	"wakebit/internal/install"
	"wakebit/internal/platform"
	"wakebit/internal/tray"
)

func main() {
	fs := flag.NewFlagSet("wakebit", flag.ContinueOnError)
	var usage strings.Builder
	fs.SetOutput(&usage)
	doInstall := fs.Bool("install", false, "install to the user profile, enable autostart and start")
	doUninstall := fs.Bool("uninstall", false, "disable autostart and remove the installed copy")
	if err := fs.Parse(os.Args[1:]); err != nil {
		reportError(strings.TrimSpace(usage.String()))
		os.Exit(2)
	}
	if *doInstall && *doUninstall {
		reportError("--install and --uninstall cannot be used together")
		os.Exit(2)
	}
	if *doInstall || *doUninstall {
		run := runInstall
		if *doUninstall {
			run = runUninstall
		}
		if err := run(); err != nil {
			reportError(err.Error())
			os.Exit(1)
		}
		return
	}

	release, ok := acquireSingleInstance()
	if !ok {
		return
	}
	defer release()

	cfgPath := config.DefaultPath()
	eng := engine.New(platform.New(), os.TempDir())
	defer eng.Stop()
	log.Printf("wakebit: starting, config %s", cfgPath)
	tray.Run(cfgPath, eng, tray.Autostart{
		Enabled: install.AutostartEnabled,
		Set: func(on bool) error {
			exe, err := os.Executable()
			if err != nil {
				return err
			}
			return install.SetAutostart(exe, on)
		},
	})
}

// Wakebit keeps the computer awake from the system tray.
package main

import (
	"flag"
	"log"
	"os"

	"wakebit/internal/config"
	"wakebit/internal/engine"
	"wakebit/internal/install"
	"wakebit/internal/platform"
	"wakebit/internal/tray"
)

func main() {
	doInstall := flag.Bool("install", false, "install to the user profile, enable autostart and start")
	doUninstall := flag.Bool("uninstall", false, "disable autostart and remove the installed copy")
	flag.Parse()
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

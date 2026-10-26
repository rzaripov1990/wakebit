// Wakebit keeps the computer awake from the system tray.
package main

import (
	"log"
	"os"

	"wakebit/internal/config"
	"wakebit/internal/engine"
	"wakebit/internal/platform"
	"wakebit/internal/tray"
)

func main() {
	release, ok := acquireSingleInstance()
	if !ok {
		return
	}
	defer release()

	cfgPath := config.DefaultPath()
	eng := engine.New(platform.New(), os.TempDir())
	defer eng.Stop()
	log.Printf("wakebit: starting, config %s", cfgPath)
	tray.Run(cfgPath, eng)
}

// Command selftest runs one Full-mode cycle of the engine on the real
// platform so it can be checked by hand on a desktop (it moves focus, opens
// Notepad and types the time). Do not touch mouse or keyboard while it runs:
// the engine only acts after 5 s without user input.
//
//	GOOS=windows go build -o selftest.exe ./cmd/selftest && selftest.exe
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"wakebit/internal/config"
	"wakebit/internal/engine"
	"wakebit/internal/platform"
)

func main() {
	dur := flag.Duration("d", 8*time.Second, "how long to let the engine run")
	flag.Parse()
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	p := platform.New()
	x, y, err := p.CursorPos()
	log.Printf("cursor at (%d,%d) err=%v", x, y, err)
	log.Printf("idle for %v", p.IdleFor())

	tmp := os.TempDir()
	log.Printf("starting Full mode, interval 1s, for %v; file %s", *dur, filepath.Join(tmp, "wakebit-time.txt"))
	log.Printf("hands off mouse and keyboard: a cycle starts once idle >= 5s")
	e := engine.New(p, tmp)
	e.Apply(config.ModeFull, true, time.Second)

	end := time.Now().Add(*dur)
	for time.Now().Before(end) {
		time.Sleep(time.Second)
		log.Printf("idle for %v", p.IdleFor())
	}
	e.Stop()
	log.Printf("stopped; sleep blocking released")
	fmt.Println("Expected: Notepad shows wakebit-time.txt with the current time typed after the first line.")
}

package main

import winplat "wakebit/internal/platform/windows"

// earlyInit runs first in main, before any window or tray icon exists.
func earlyInit() { winplat.EnsureDPIAware() }

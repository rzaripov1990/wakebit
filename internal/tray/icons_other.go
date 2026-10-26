//go:build !windows

package tray

func encodeIcon(pngData []byte) []byte { return pngData }

package tray

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"
)

// ConsoleURL returns the browser URL for the Console given an HTTP bind address.
func ConsoleURL(httpBind string) string {
	return fmt.Sprintf("http://%s/console/", httpBind)
}

// OpenURL opens url in the default browser on Windows and macOS.
// On Linux it logs and returns nil (Open Console is omitted from the tray).
func OpenURL(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		log.Printf("acc: OpenURL unsupported on %s (%s)", runtime.GOOS, url)
		return nil
	}
}

package web

import (
	"os/exec"
	"runtime"
)

// OpenBrowser opens url in the system's default browser, best effort:
// failures are ignored because the server keeps running regardless and the
// URL is printed to the terminal.
func OpenBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

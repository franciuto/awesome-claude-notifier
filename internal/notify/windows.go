//go:build windows

package notify

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

type windows struct{}

// New returns the notifier for the current OS.
func New() Notifier { return windows{} }

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

// start runs toast.ps1 with the request in the environment, not on the command
// line, so quotes in the text stay text. Windows PowerShell, not pwsh: only 5.1
// can load the WinRT notification types.
func start(req map[string]any) *exec.Cmd {
	script := filepath.Join(PluginRoot(), "windows", "toast.ps1")
	if !fileExists(script) {
		return nil
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script)
	cmd.Env = append(os.Environ(), "CLAUDE_NOTIFY_REQUEST="+string(data))
	// CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000 | 0x00000200}
	if cmd.Start() != nil {
		return nil
	}
	return cmd
}

// Send returns once the script is running; it keeps going to play the sound.
func (windows) Send(r Request) bool {
	cmd := start(toastRequest(r, "", 0, newID(), "", 0, PluginRoot(), CacheDir(), fileExists))
	if cmd == nil {
		return false
	}
	go cmd.Wait()
	return true
}

// Ask has no signal forwarding as on macOS: Windows kills a process without running
// handlers, so the script withdraws the toast itself once this process is gone.
func (windows) Ask(r Request, actions string, wait time.Duration) *Answer {
	dir := filepath.Join(CacheDir(), "responses")
	if os.MkdirAll(dir, 0o755) != nil {
		return nil
	}
	id := newID()
	answerFile := filepath.Join(dir, id+".json")
	cmd := start(toastRequest(r, actions, wait, id, answerFile, os.Getpid(), PluginRoot(), CacheDir(), fileExists))
	if cmd == nil {
		return nil
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var err error
	select {
	case err = <-done:
	case <-time.After(wait + 15*time.Second):
		cmd.Process.Kill()
		err = <-done
	}
	return readAnswer(answerFile, err == nil)
}

func newID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6], b[8] = b[6]&0x0f|0x40, b[8]&0x3f|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}

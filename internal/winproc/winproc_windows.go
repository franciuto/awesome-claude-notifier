package winproc

import (
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Processes returns pid -> parent and exe name for every running process.
func Processes() map[uint32]Proc {
	procs := map[uint32]Proc{}
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return procs
	}
	defer windows.CloseHandle(snap)
	e := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		procs[e.ProcessID] = Proc{Parent: e.ParentProcessID, Exe: windows.UTF16ToString(e.ExeFile[:])}
	}
	return procs
}

// ForegroundApp is the pid and lowercase exe name of the app owning the foreground window.
func ForegroundApp() (pid uint32, exe string, procs map[uint32]Proc, err error) {
	hwnd := windows.GetForegroundWindow()
	if hwnd == 0 {
		return 0, "", nil, windows.ERROR_INVALID_WINDOW_HANDLE
	}
	if _, err = windows.GetWindowThreadProcessId(hwnd, &pid); err != nil {
		return 0, "", nil, err
	}
	procs = Processes()
	return pid, strings.ToLower(procs[pid].Exe), procs, nil
}

// StartedAt is when the process started; false if it isn't running.
func StartedAt(pid uint32) (time.Time, bool) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return time.Time{}, false
	}
	defer windows.CloseHandle(h)
	var created, exited, kernel, user windows.Filetime
	if windows.GetProcessTimes(h, &created, &exited, &kernel, &user) != nil {
		return time.Time{}, false
	}
	return time.Unix(0, created.Nanoseconds()), true
}

// Terminate ends the process with exit code 0, which Claude Code treats as a quiet hook exit.
func Terminate(pid uint32) {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return
	}
	defer windows.CloseHandle(h)
	windows.TerminateProcess(h, 0)
}

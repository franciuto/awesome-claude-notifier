"""Windows side: the toast script and the process lookups the hook needs, via the Win32 API

Import only on Windows.
"""

import ctypes
from ctypes import wintypes
from pathlib import Path
from typing import Dict, List, Optional, Tuple

# Shows the toast and waits for Allow / Deny / Reply; see the header of the script
TOAST_SCRIPT = Path(__file__).with_name("toast.ps1")
ICON = Path(__file__).with_name("icon.png")

_kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)
_user32 = ctypes.WinDLL("user32", use_last_error=True)

_TH32CS_SNAPPROCESS = 0x2
_PROCESS_TERMINATE = 0x1
_PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
_INVALID_HANDLE = wintypes.HANDLE(-1).value
# FILETIME counts 100 ns steps from 1601, Unix time seconds from 1970
_EPOCH_AS_FILETIME = 116444736000000000


class _PROCESSENTRY32W(ctypes.Structure):
    _fields_ = [
        ("dwSize", wintypes.DWORD),
        ("cntUsage", wintypes.DWORD),
        ("th32ProcessID", wintypes.DWORD),
        ("th32DefaultHeapID", ctypes.c_size_t),
        ("th32ModuleID", wintypes.DWORD),
        ("cntThreads", wintypes.DWORD),
        ("th32ParentProcessID", wintypes.DWORD),
        ("pcPriClassBase", ctypes.c_long),
        ("dwFlags", wintypes.DWORD),
        ("szExeFile", wintypes.WCHAR * 260),
    ]


_kernel32.CreateToolhelp32Snapshot.restype = wintypes.HANDLE
_kernel32.CreateToolhelp32Snapshot.argtypes = [wintypes.DWORD, wintypes.DWORD]
_kernel32.Process32FirstW.argtypes = [wintypes.HANDLE, ctypes.POINTER(_PROCESSENTRY32W)]
_kernel32.Process32NextW.argtypes = [wintypes.HANDLE, ctypes.POINTER(_PROCESSENTRY32W)]
_kernel32.OpenProcess.restype = wintypes.HANDLE
_kernel32.OpenProcess.argtypes = [wintypes.DWORD, wintypes.BOOL, wintypes.DWORD]
_kernel32.CloseHandle.argtypes = [wintypes.HANDLE]
_kernel32.GetProcessTimes.argtypes = [wintypes.HANDLE] + [ctypes.POINTER(wintypes.FILETIME)] * 4
_kernel32.TerminateProcess.argtypes = [wintypes.HANDLE, wintypes.UINT]
_user32.GetForegroundWindow.restype = wintypes.HWND
_user32.GetWindowThreadProcessId.argtypes = [wintypes.HWND, ctypes.POINTER(wintypes.DWORD)]


def _processes() -> Dict[int, Tuple[int, str]]:
    """pid -> (parent pid, exe name) for every running process"""
    snapshot = _kernel32.CreateToolhelp32Snapshot(_TH32CS_SNAPPROCESS, 0)
    if not snapshot or snapshot == _INVALID_HANDLE:
        return {}
    processes = {}
    try:
        entry = _PROCESSENTRY32W()
        entry.dwSize = ctypes.sizeof(entry)
        ok = _kernel32.Process32FirstW(snapshot, ctypes.byref(entry))
        while ok:
            processes[entry.th32ProcessID] = (entry.th32ParentProcessID, entry.szExeFile)
            ok = _kernel32.Process32NextW(snapshot, ctypes.byref(entry))
    finally:
        _kernel32.CloseHandle(snapshot)
    return processes


def ancestors(pid: int, processes: Optional[Dict[int, Tuple[int, str]]] = None) -> List[int]:
    """Parent, grandparent, ... of pid"""
    processes = processes if processes is not None else _processes()
    chain: List[int] = []
    while pid in processes and len(chain) < 32:
        pid = processes[pid][0]
        if not pid or pid in chain:
            break
        chain.append(pid)
    return chain


def foreground_app() -> Tuple[Optional[int], str, Dict[int, Tuple[int, str]]]:
    """Pid and exe name (lowercase) of the app owning the foreground window, plus the process table"""
    pid = wintypes.DWORD()
    hwnd = _user32.GetForegroundWindow()
    if not hwnd or not _user32.GetWindowThreadProcessId(hwnd, ctypes.byref(pid)):
        return None, "", {}
    processes = _processes()
    return pid.value, processes.get(pid.value, (0, ""))[1].lower(), processes


def process_started_at(pid: int) -> Optional[float]:
    """Unix time the process started, or None if it isn't running"""
    handle = _kernel32.OpenProcess(_PROCESS_QUERY_LIMITED_INFORMATION, False, pid)
    if not handle:
        return None
    try:
        created, exited, kernel, user = (wintypes.FILETIME() for _ in range(4))
        if not _kernel32.GetProcessTimes(handle, ctypes.byref(created), ctypes.byref(exited),
                                         ctypes.byref(kernel), ctypes.byref(user)):
            return None
        filetime = (created.dwHighDateTime << 32) | created.dwLowDateTime
        return (filetime - _EPOCH_AS_FILETIME) / 1e7
    finally:
        _kernel32.CloseHandle(handle)


def terminate(pid: int) -> None:
    """End the process with exit code 0, which Claude Code treats as a quiet hook exit"""
    handle = _kernel32.OpenProcess(_PROCESS_TERMINATE, False, pid)
    if handle:
        try:
            _kernel32.TerminateProcess(handle, 0)
        finally:
            _kernel32.CloseHandle(handle)

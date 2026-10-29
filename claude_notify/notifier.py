"""Cross-platform notification system for Claude"""

import json
import platform
import shutil
import signal
import subprocess
import os
import sys
import uuid
from pathlib import Path
from typing import Optional, Dict, Any

# Helper app that posts notifications with its own (yellow) icon; see helper/build.sh
MACOS_HELPER_APP = os.path.expanduser("~/Applications/Claude Notify.app")
MACOS_HELPER = os.path.join(MACOS_HELPER_APP, "Contents/MacOS/claude-notify-helper")

if sys.platform == "win32":
    _APP_DIR = Path(os.environ.get("LOCALAPPDATA") or Path.home() / "AppData/Local") / "claude-notify"
    CACHE_DIR = _APP_DIR / "cache"
    # Group sounds, see sounds/install.ps1
    SOUNDS_DIR = _APP_DIR / "sounds"
    SOUND_EXT = ".mp3"
else:
    CACHE_DIR = Path.home() / "Library/Caches/claude-notify"
    # Group sounds, see sounds/install.sh
    SOUNDS_DIR = Path.home() / "Library/Sounds"
    SOUND_EXT = ".aiff"

# Where the helper writes the answer to an interactive notification
HELPER_RESPONSES_DIR = CACHE_DIR / "responses"

# Keep the toast script's PowerShell window from flashing up, and Ctrl+C in the terminal away from it
WINDOWS_CREATION_FLAGS = (getattr(subprocess, "CREATE_NO_WINDOW", 0)
                          | getattr(subprocess, "CREATE_NEW_PROCESS_GROUP", 0))


class ClaudeNotifier:
    """Cross-platform notification handler for Claude alerts"""
    
    def __init__(self, app_name: str = "Claude"):
        self.app_name = app_name
        self.system = platform.system().lower()
        
    def send_notification(
        self, 
        title: str, 
        message: str, 
        urgency: str = "normal",
        timeout: int = 10,
        sound: bool = True,
        sound_name: Optional[str] = None
    ) -> bool:
        """
        Send a notification across different platforms

        Args:
            title: Notification title
            message: Notification message
            urgency: Urgency level (low, normal, critical)
            timeout: Notification timeout in seconds
            sound: Whether to play a sound
            sound_name: macOS and Windows, a file in SOUNDS_DIR (e.g. "claude-notify-done.aiff")

        Returns:
            bool: True if notification was sent successfully
        """
        try:
            if self.system == "darwin":  # macOS
                return self._send_macos_notification(title, message, sound, sound_name)
            elif self.system == "linux":
                return self._send_linux_notification(title, message, urgency, timeout)
            elif self.system == "windows":
                return self._send_windows_notification(title, message, sound, sound_name)
            return False
        except Exception as e:
            print(f"Notification error: {e}", file=sys.stderr)
            return False
    
    def ask(self, title: str, message: str, actions: str, wait: int,
            sound_name: Optional[str] = None) -> Optional[Dict[str, str]]:
        """
        Show an interactive notification (macOS helper or Windows toast) and wait for the answer

        Args:
            actions: "permission" (Allow / Deny) or "reply" (Reply)
            wait: seconds to wait before giving up and withdrawing the notification

        Returns:
            {"action": "allow" | "deny" | "reply" | "open" | "timeout", "text": ...},
            or None if the helper is unavailable or failed
        """
        if self.system == "windows":
            return self._ask_windows(title, message, actions, wait, sound_name)
        if self.system != "darwin" or not os.access(MACOS_HELPER, os.X_OK):
            return None

        # Launched through LaunchServices, not exec'd: only then does macOS treat the
        # helper as the app and route the click (and the typed reply) back to it.
        request_id = str(uuid.uuid4())
        answer_file = HELPER_RESPONSES_DIR / f"{request_id}.json"
        pid_file = HELPER_RESPONSES_DIR / f"{request_id}.pid"
        cmd = ["open", "-n", "-g", "-W", "-a", MACOS_HELPER_APP, "--args",
               "--title", title, "--body", message,
               "--actions", actions, "--wait", str(wait), "--id", request_id,
               "--parent", str(os.getpid())]
        if sound_name:
            cmd += ["--sound", sound_name]
        child = subprocess.Popen(cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

        def stop_helper():
            try:
                os.kill(int(pid_file.read_text().strip()), signal.SIGTERM)
            except (OSError, ValueError):
                pass

        # If we get killed (answer no longer wanted), take the notification with us
        def forward(signum, frame):
            stop_helper()
            try:
                child.wait(timeout=3)
            finally:
                sys.exit(0)

        previous = {sig: signal.signal(sig, forward) for sig in (signal.SIGTERM, signal.SIGINT, signal.SIGHUP)}
        try:
            child.wait(timeout=wait + 15)
        except subprocess.TimeoutExpired:
            stop_helper()
            child.kill()
        finally:
            for sig, handler in previous.items():
                signal.signal(sig, handler)

        return self._read_answer(answer_file, child)

    def _ask_windows(self, title: str, message: str, actions: str, wait: int,
                     sound_name: Optional[str]) -> Optional[Dict[str, str]]:
        """ask() on Windows: the toast script waits for the click and writes the answer file"""
        HELPER_RESPONSES_DIR.mkdir(parents=True, exist_ok=True)
        request_id = str(uuid.uuid4())
        answer_file = HELPER_RESPONSES_DIR / f"{request_id}.json"
        # No signal forwarding as on macOS: Windows kills a process without running handlers,
        # so the script withdraws the toast by itself once this process ("parent") is gone.
        child = self._start_windows_toast({
            "title": title, "body": message, "sound": self._windows_sound(True, sound_name),
            "actions": actions, "wait": wait, "id": request_id,
            "response": str(answer_file), "parent": os.getpid(),
        })
        if child is None:
            return None
        try:
            child.wait(timeout=wait + 15)
        except subprocess.TimeoutExpired:
            child.kill()
            child.wait()
        return self._read_answer(answer_file, child)

    @staticmethod
    def _read_answer(answer_file: Path, child: subprocess.Popen) -> Optional[Dict[str, str]]:
        """The answer the helper wrote; no file after a clean exit means nobody answered"""
        try:
            answer = json.loads(answer_file.read_text(encoding="utf-8"))
            answer_file.unlink()
            # The macOS helper failed; `open -W` exits 0 regardless, so it says so in the file
            return None if answer.get("action") == "error" else answer
        except FileNotFoundError:
            return {"action": "timeout"} if child.returncode == 0 else None
        except (OSError, ValueError):
            return None

    def _send_macos_notification(self, title: str, message: str, sound: bool,
                                 sound_name: Optional[str] = None) -> bool:
        """Send notification on macOS via the Claude Notify helper app, else osascript"""
        if os.access(MACOS_HELPER, os.X_OK):
            cmd = [MACOS_HELPER, "--title", title, "--body", message]
            if not sound:
                cmd += ["--sound", "none"]
            elif sound_name:
                cmd += ["--sound", sound_name]
            try:
                subprocess.run(cmd, check=True, capture_output=True, text=True, timeout=15)
                return True
            except (subprocess.CalledProcessError, subprocess.TimeoutExpired):
                pass  # fall back to osascript below

        # Escape for AppleScript string literals so quotes in the text can't break out
        title = title.replace("\\", "\\\\").replace('"', '\\"')
        message = message.replace("\\", "\\\\").replace('"', '\\"')
        sound_arg = "with sound" if sound else ""
        script = f'''
        display notification "{message}" with title "{title}" subtitle "{self.app_name}" {sound_arg}
        '''
        
        try:
            subprocess.run(
                ["osascript", "-e", script],
                check=True,
                capture_output=True,
                text=True
            )
            return True
        except subprocess.CalledProcessError:
            return False
    
    def _send_linux_notification(
        self, 
        title: str, 
        message: str, 
        urgency: str, 
        timeout: int
    ) -> bool:
        """Send notification on Linux using notify-send"""
        try:
            cmd = [
                "notify-send",
                f"--app-name={self.app_name}",
                f"--urgency={urgency}",
                f"--expire-time={timeout * 1000}",  # Convert to milliseconds
                title,
                message
            ]
            
            subprocess.run(cmd, check=True)
            return True
        except (subprocess.CalledProcessError, FileNotFoundError):
            return False
    
    def _send_windows_notification(self, title: str, message: str, sound: bool,
                                   sound_name: Optional[str] = None) -> bool:
        """Send a toast on Windows; the script keeps running in the background to play the sound"""
        child = self._start_windows_toast({
            "title": title, "body": message, "sound": self._windows_sound(sound, sound_name),
            "actions": "", "id": str(uuid.uuid4()),
        })
        return child is not None

    @staticmethod
    def _windows_sound(sound: bool, sound_name: Optional[str]) -> str:
        """The toast script's "sound": "none", an mp3 path, or "default" for the Windows sound"""
        if not sound:
            return "none"
        if sound_name and (SOUNDS_DIR / sound_name).exists():
            return str(SOUNDS_DIR / sound_name)
        return "default"

    @staticmethod
    def _start_windows_toast(request: Dict[str, Any]) -> Optional[subprocess.Popen]:
        """Start the toast script (claude_notify/windows/toast.ps1) with this request"""
        from . import windows

        if not windows.TOAST_SCRIPT.exists():
            return None
        request.update(icon=str(windows.ICON), cache=str(CACHE_DIR))
        # Passed in the environment, not on the command line, so quotes in the text stay text
        env = dict(os.environ, CLAUDE_NOTIFY_REQUEST=json.dumps(request))
        # Windows PowerShell, not pwsh: only 5.1 can load the WinRT notification types
        cmd = ["powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
               "-File", str(windows.TOAST_SCRIPT)]
        return subprocess.Popen(cmd, env=env, stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL,
                                stderr=subprocess.DEVNULL, creationflags=WINDOWS_CREATION_FLAGS)
    
    def check_dependencies(self) -> Dict[str, bool]:
        """Check which notification methods are available"""
        deps = {"native": False}
        
        if self.system == "darwin":
            # macOS always has osascript
            deps["native"] = True
            deps["method"] = "osascript"
        elif self.system == "linux":
            deps["native"] = shutil.which("notify-send") is not None
            deps["method"] = "notify-send"
        elif self.system == "windows":
            # Toasts from Windows PowerShell 5.1, part of every Windows 10 and 11
            deps["native"] = True
            deps["method"] = "toast (powershell.exe)"
        
        return deps
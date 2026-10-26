"""Cross-platform notification system for Claude"""

import json
import platform
import signal
import subprocess
import os
import sys
import uuid
from pathlib import Path
from typing import Optional, Dict, Any
from plyer import notification as plyer_notification

# Helper app that posts notifications with its own (yellow) icon; see helper/build.sh
MACOS_HELPER_APP = os.path.expanduser("~/Applications/Claude Notify.app")
MACOS_HELPER = os.path.join(MACOS_HELPER_APP, "Contents/MacOS/claude-notify-helper")

# Where the helper writes the answer to an interactive notification
HELPER_RESPONSES_DIR = Path.home() / "Library/Caches/claude-notify/responses"


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
            sound_name: macOS only, a file in ~/Library/Sounds (e.g. "claude-notify-done.aiff")

        Returns:
            bool: True if notification was sent successfully
        """
        try:
            if self.system == "darwin":  # macOS
                return self._send_macos_notification(title, message, sound, sound_name)
            elif self.system == "linux":
                return self._send_linux_notification(title, message, urgency, timeout)
            elif self.system == "windows":
                return self._send_windows_notification(title, message, timeout)
            else:
                # Fallback to plyer for unknown systems
                return self._send_plyer_notification(title, message, timeout)
        except Exception as e:
            print(f"Notification error: {e}")
            # Try fallback method
            return self._send_plyer_notification(title, message, timeout)
    
    def ask(self, title: str, message: str, actions: str, wait: int,
            sound_name: Optional[str] = None) -> Optional[Dict[str, str]]:
        """
        Show an interactive notification (macOS helper only) and wait for the answer

        Args:
            actions: "permission" (Allow / Deny) or "reply" (Reply)
            wait: seconds to wait before giving up and withdrawing the notification

        Returns:
            {"action": "allow" | "deny" | "reply" | "open" | "timeout", "text": ...},
            or None if the helper is unavailable or failed
        """
        if self.system != "darwin" or not os.access(MACOS_HELPER, os.X_OK):
            return None

        # Launched through LaunchServices, not exec'd: only then does macOS treat the
        # helper as the app and route the click (and the typed reply) back to it.
        request_id = str(uuid.uuid4())
        answer_file = HELPER_RESPONSES_DIR / f"{request_id}.json"
        pid_file = HELPER_RESPONSES_DIR / f"{request_id}.pid"
        cmd = ["open", "-n", "-g", "-W", "-a", MACOS_HELPER_APP, "--args",
               "--title", title, "--body", message,
               "--actions", actions, "--wait", str(wait), "--id", request_id]
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

        try:
            answer = json.loads(answer_file.read_text())
            answer_file.unlink()
            return answer
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
            # Check if notify-send is available
            subprocess.run(
                ["which", "notify-send"],
                check=True,
                capture_output=True
            )
            
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
    
    def _send_windows_notification(self, title: str, message: str, timeout: int) -> bool:
        """Send notification on Windows using PowerShell"""
        try:
            ps_script = f'''
            [Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
            [Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom, ContentType = WindowsRuntime] | Out-Null
            
            $template = @"
            <toast duration="long">
                <visual>
                    <binding template="ToastText02">
                        <text id="1">{title}</text>
                        <text id="2">{message}</text>
                    </binding>
                </visual>
                <audio src="ms-winsoundevent:Notification.Default" />
            </toast>
"@
            
            $xml = New-Object Windows.Data.Xml.Dom.XmlDocument
            $xml.LoadXml($template)
            $toast = New-Object Windows.UI.Notifications.ToastNotification $xml
            [Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("{self.app_name}").Show($toast)
            '''
            
            subprocess.run(
                ["powershell", "-Command", ps_script],
                check=True,
                capture_output=True
            )
            return True
        except subprocess.CalledProcessError:
            return False
    
    def _send_plyer_notification(self, title: str, message: str, timeout: int) -> bool:
        """Fallback notification using plyer library"""
        try:
            plyer_notification.notify(
                title=title,
                message=message,
                app_name=self.app_name,
                timeout=timeout
            )
            return True
        except Exception:
            return False
    
    def check_dependencies(self) -> Dict[str, bool]:
        """Check which notification methods are available"""
        deps = {
            "plyer": True,  # Always available if installed
            "native": False
        }
        
        if self.system == "darwin":
            # macOS always has osascript
            deps["native"] = True
            deps["method"] = "osascript"
        elif self.system == "linux":
            # Check for notify-send
            try:
                subprocess.run(
                    ["which", "notify-send"],
                    check=True,
                    capture_output=True
                )
                deps["native"] = True
                deps["method"] = "notify-send"
            except subprocess.CalledProcessError:
                deps["method"] = "plyer"
        elif self.system == "windows":
            # Windows with PowerShell
            deps["native"] = True
            deps["method"] = "powershell"
        
        return deps
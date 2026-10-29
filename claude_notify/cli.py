"""Command-line interface for Claude Notify"""

import click
import json
import sys
from typing import Optional
from .notifier import ClaudeNotifier
from .hook_handler import HookHandler


@click.group()
@click.version_option(package_name="claude-notify")
def cli():
    """Claude Notify - Cross-platform notifications for Claude"""
    pass


@cli.command()
@click.option(
    "--title", "-t",
    default="Claude needs your attention",
    help="Notification title"
)
@click.option(
    "--message", "-m",
    default="Claude is waiting for your response",
    help="Notification message"
)
@click.option(
    "--urgency", "-u",
    type=click.Choice(["low", "normal", "critical"]),
    default="normal",
    help="Notification urgency level"
)
@click.option(
    "--timeout", "-s",
    type=int,
    default=10,
    help="Notification timeout in seconds"
)
@click.option(
    "--sound/--no-sound",
    default=True,
    help="Play notification sound"
)
def send(title: str, message: str, urgency: str, timeout: int, sound: bool):
    """Send a notification immediately"""
    notifier = ClaudeNotifier()
    
    success = notifier.send_notification(
        title=title,
        message=message,
        urgency=urgency,
        timeout=timeout,
        sound=sound
    )
    
    if success:
        click.echo("✓ Notification sent successfully")
    else:
        click.echo("✗ Failed to send notification", err=True)
        sys.exit(1)


@cli.command()
def check():
    """Check notification system dependencies"""
    notifier = ClaudeNotifier()
    deps = notifier.check_dependencies()
    
    click.echo("Claude Notify System Check")
    click.echo("=" * 30)
    click.echo(f"Operating System: {notifier.system}")
    click.echo(f"Native support: {'✓' if deps['native'] else '✗'}")
    click.echo(f"Notification method: {deps.get('method', 'unknown')}")
    
    # Test notification
    click.echo("\nSending test notification...")
    success = notifier.send_notification(
        title="Claude Notify Test",
        message="This is a test notification",
        timeout=5
    )
    
    if success:
        click.echo("✓ Test notification sent successfully")
    else:
        click.echo("✗ Test notification failed", err=True)


@cli.command()
@click.option(
    "--event-type", "-e",
    help="Override the event type (PreToolUse, PostToolUse, Notification, Stop, SubagentStop)"
)
@click.option(
    "--reply-wait",
    type=int,
    default=0,
    help="Stop/Notification: offer Reply and wait this many seconds for a reply (use with asyncRewake)"
)
def hook(event_type: Optional[str], reply_wait: int):
    """
    Process Claude Code hook events from JSON input
    
    This command reads JSON from stdin and sends notifications based on the hook event.
    It's designed to be used in Claude Code hook configurations.
    
    Example usage in settings.json:
    
    \b
    {
      "hooks": {
        "PreToolUse": [{
          "matcher": ".*",
          "hooks": [{
            "type": "command",
            "command": "claude-notify hook"
          }]
        }]
      }
    }
    """
    # Claude Code talks UTF-8; on Windows pipes default to the ANSI code page
    for stream in (sys.stdin, sys.stdout, sys.stderr):
        if hasattr(stream, "reconfigure"):
            stream.reconfigure(encoding="utf-8")

    handler = HookHandler()

    data = handler.read_stdin_json()
    if not isinstance(data, dict) or not data:
        click.echo("Error: No JSON data received from stdin", err=True)
        sys.exit(1)

    event_type = event_type or data.get("hook_event_name")
    if not event_type:
        click.echo("Error: no hook_event_name in the JSON data; pass --event-type", err=True)
        sys.exit(1)
    
    # Allow / Deny from the notification; no output leaves it to the terminal dialog
    if event_type == "PermissionRequest":
        output = handler.handle_permission_request(data)
        if output:
            click.echo(json.dumps(output))
        sys.exit(0)

    # A prompt typed in the terminal makes a pending Reply pointless
    if event_type == "UserPromptSubmit":
        handler.cancel_reply_wait(data.get("session_id", "unknown"))
        sys.exit(0)

    # Reply: exit code 2 makes an asyncRewake hook wake Claude with stderr
    if reply_wait > 0 and not handler.in_desktop_app():
        reply = handler.notify_and_wait_reply(event_type, data, reply_wait)
        if reply:
            click.echo(
                "Message from the user, sent by replying to a desktop notification. "
                "Treat it as a new message from them and answer normally:\n\n" + reply,
                err=True,
            )
            sys.exit(2)
        sys.exit(0)

    # Process the hook event
    success = handler.process_hook_event(event_type, data)
    
    if not success:
        click.echo("Warning: Failed to send notification", err=True)
        # Don't exit with error code to avoid blocking Claude operations
    
    # Exit successfully to not block Claude
    sys.exit(0)


def main():
    """Main entry point"""
    try:
        cli(standalone_mode=False)
    except click.ClickException as e:
        # Click exits usage errors with 2, which Claude Code treats as blocking (or as a Reply)
        e.show()
        sys.exit(1)


if __name__ == "__main__":
    main()
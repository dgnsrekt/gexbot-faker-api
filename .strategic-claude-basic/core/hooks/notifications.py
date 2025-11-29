#!/usr/bin/python3
"""
Notification utilities for Claude Code hooks.

Shared functions for sending ntfy notifications and getting project context
across all Claude hooks.
"""

import os
import subprocess
import sys
from datetime import datetime
from pathlib import Path
from typing import Optional, List

# Configuration constants
PROJECT_NAME = "strategic-claude-basic"
PROJECT_TITLE = PROJECT_NAME.replace('-', ' ').title()
PROJECT_TAG = PROJECT_NAME.lower()
NTFY_SERVER_URL = "http://nas1-oryx:2586"
NTFY_TOPIC = f"{PROJECT_NAME}"
PROJECT_TYPE = "Claude project"


def log(message: str, hook_name: str = "notifications") -> None:
    """Print a message to stderr to avoid interfering with hook output."""
    print(f"[{hook_name}] {message}", file=sys.stderr)


def play_audio(audio_file: str, hook_name: str = "notifications") -> bool:
    """
    Play an audio file using system audio player.

    Args:
        audio_file: Name of the audio file (e.g., "toasty.mp3", "gutter-trash.mp3")
        hook_name: Name of the calling hook (for logging)

    Returns:
        True if audio played successfully, False otherwise
    """
    try:
        # Get the path to the assets directory relative to this script
        script_dir = Path(__file__).parent
        assets_dir = script_dir / "assets"
        audio_path = assets_dir / audio_file

        if not audio_path.exists():
            log(f"Audio file not found: {audio_path}", hook_name)
            return False

        # Try different audio players based on the system
        audio_players = [
            ["afplay"],  # macOS
            ["mpg123", "-q"],  # Linux (quiet mode)
            ["mpv", "--no-video", "--really-quiet"],  # Cross-platform
            ["ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet"],  # ffmpeg
            ["paplay"],  # PulseAudio
        ]

        for player_cmd in audio_players:
            try:
                # Check if the player is available
                result = subprocess.run(
                    ["which", player_cmd[0]], capture_output=True, timeout=2
                )

                if result.returncode == 0:
                    # Player is available, try to play the audio
                    cmd = player_cmd + [str(audio_path)]
                    subprocess.run(
                        cmd,
                        capture_output=True,
                        timeout=5,  # Don't let audio play longer than 5 seconds
                        check=False,  # Don't raise exception on non-zero exit
                    )
                    log(f"♪ Played audio: {audio_file}", hook_name)
                    return True
            except (
                subprocess.TimeoutExpired,
                subprocess.CalledProcessError,
                FileNotFoundError,
            ):
                continue

        log(f"No suitable audio player found for: {audio_file}", hook_name)
        return False

    except Exception as e:
        log(f"Error playing audio {audio_file}: {e}", hook_name)
        return False


def send_ntfy_notification(
    server_url: str,
    topic: str,
    message: str,
    title: Optional[str] = None,
    priority: str = "default",
    tags: Optional[List[str]] = None,
    hook_name: str = "notifications",
) -> bool:
    """
    Send notification to ntfy server.

    Args:
        server_url: The ntfy server URL (e.g., "http://localhost:2586")
        topic: The topic to send to
        message: The notification message
        title: Optional notification title
        priority: Notification priority (default, low, high, max)
        tags: Optional list of tags for the notification
        hook_name: Name of the calling hook (for logging)

    Returns:
        True if notification was sent successfully, False otherwise
    """
    try:
        headers = []

        if title:
            headers.extend(["-H", f"Title: {title}"])

        if priority != "default":
            headers.extend(["-H", f"Priority: {priority}"])

        if tags:
            headers.extend(["-H", f"Tags: {','.join(tags)}"])

        # Add timestamp header
        headers.extend(["-H", f"X-Timestamp: {datetime.now().isoformat()}"])

        # Construct curl command
        cmd = ["curl", "-s", "-f"] + headers + ["-d", message, f"{server_url}/{topic}"]

        result = subprocess.run(cmd, capture_output=True, text=True, timeout=10)

        if result.returncode == 0:
            log(f"✓ Notification sent successfully to {topic}", hook_name)
            return True
        else:
            log(f"✗ Failed to send notification: {result.stderr}", hook_name)
            return False

    except subprocess.TimeoutExpired:
        log("✗ Notification timeout - server may be unreachable", hook_name)
        return False
    except Exception as e:
        log(f"✗ Error sending notification: {e}", hook_name)
        log(f"✗ Exception type: {type(e).__name__}", hook_name)
        log(f"✗ Exception details: {str(e)}", hook_name)
        return False


def get_project_context(cwd: str, hook_name: str = "notifications") -> dict:
    """
    Get context about the current project.

    Args:
        cwd: Current working directory path
        hook_name: Name of the calling hook (for logging)

    Returns:
        Dictionary with project information including name and git status
    """
    try:
        project_path = Path(cwd)
        project_name = project_path.name

        # Check git status
        git_status = "unknown"
        try:
            result = subprocess.run(
                ["git", "status", "--porcelain"],
                cwd=cwd,
                capture_output=True,
                text=True,
                timeout=5,
            )
            if result.returncode == 0:
                changes = (
                    len(result.stdout.strip().split("\n"))
                    if result.stdout.strip()
                    else 0
                )
                git_status = f"{changes} changes" if changes > 0 else "clean"
        except Exception as e:
            log(f"Git status check failed: {e}", hook_name)

        # Get current git branch
        git_branch = "main"
        try:
            result = subprocess.run(
                ["git", "branch", "--show-current"],
                cwd=cwd,
                capture_output=True,
                text=True,
                timeout=5,
            )
            if result.returncode == 0:
                git_branch = result.stdout.strip() or "main"
        except Exception:
            pass

        return {
            "project_name": project_name,
            "project_type": PROJECT_TYPE,
            "git_status": git_status,
            "git_branch": git_branch,
            "path": str(project_path),
        }

    except Exception as e:
        log(f"Error getting project context: {e}", hook_name)
        return {
            "project_name": "unknown",
            "project_type": PROJECT_TYPE,
            "git_status": "unknown",
            "git_branch": "main",
            "path": cwd,
        }


def get_project_config() -> dict:
    """
    Get project-specific configuration.

    Returns:
        Dictionary with ntfy server settings and topics
    """
    return {
        "ntfy_server": NTFY_SERVER_URL,
        "topic": NTFY_TOPIC,
    }


def format_message_header(project_info: dict, session_id: str) -> str:
    """
    Format a standard message header for notifications.

    Args:
        project_info: Project context from get_project_context()
        session_id: Claude Code session ID

    Returns:
        Formatted header string
    """
    return f"""**Project**: {project_info["project_name"]} ({project_info["project_type"]})
**Branch**: {project_info["git_branch"]}
**Git Status**: {project_info["git_status"]}

Session ID: {session_id[:8]}...
Path: {os.path.basename(project_info["path"])}
Time: {datetime.now().strftime("%Y-%m-%d %H:%M:%S")}"""


def send_notification(
    message: str,
    title: str,
    priority: str = "default",
    tags: Optional[List[str]] = None,
    project_info: Optional[dict] = None,
    session_id: str = "unknown",
    hook_name: str = "notifications",
) -> bool:
    """
    Send a notification with standard formatting.

    Args:
        message: The main notification message
        title: Notification title
        priority: Notification priority (default, low, high, max)
        tags: Optional list of tags
        project_info: Project context (if None, will be detected from cwd)
        session_id: Claude Code session ID
        hook_name: Name of the calling hook

    Returns:
        True if notification was sent successfully, False otherwise
    """
    config = get_project_config()
    topic = config["topic"]

    # Add project tag if not present
    project_tag = PROJECT_TAG
    if tags is None:
        tags = [project_tag]
    elif project_tag not in tags:
        tags.append(project_tag)

    # Format message with header if project info is provided
    if project_info:
        header = format_message_header(project_info, session_id)
        formatted_message = (
            f"{PROJECT_TITLE} Development\n\n{header}\n\n**Message**: {message}"
        )
    else:
        formatted_message = f"{PROJECT_TITLE}: {message}"

    return send_ntfy_notification(
        server_url=config["ntfy_server"],
        topic=topic,
        message=formatted_message,
        title=title,
        priority=priority,
        tags=tags,
        hook_name=hook_name,
    )

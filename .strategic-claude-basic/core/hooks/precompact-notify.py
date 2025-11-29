#!/usr/bin/python3
"""
PreCompact notification hook.

This hook is triggered when Claude Code's context window becomes full
and auto-compaction is triggered. It plays an audio sound
and sends an ntfy notification to alert the user.
"""

import json
import sys
from typing import Tuple, List

# Import shared notification utilities
from notifications import (
    log,
    get_project_context,
    send_notification,
    play_audio,
    PROJECT_TITLE,
    PROJECT_TAG,
)

HOOK_NAME = "precompact-notify"

# Configuration
ENABLE_COMPACT_NOTIFICATIONS = True
COMPACT_AUDIO_FILE = "get-over-here.mp3"  # Audio file to play on compact


def get_compact_notification_info() -> Tuple[str, str, List[str]]:
    """
    Get notification characteristics for auto-compact event.

    Returns:
        Tuple of (priority, title, tags)
    """
    priority = "high"
    title = f"{PROJECT_TITLE}: Auto-Compact Triggered"
    tags = ["exclamation", "fire", PROJECT_TAG, "compact"]

    return priority, title, tags


def main() -> None:
    """Main hook execution."""
    try:
        # Check if compact notifications are enabled
        if not ENABLE_COMPACT_NOTIFICATIONS:
            return

        # Read hook input
        hook_input = json.loads(sys.stdin.read())

        # Extract information
        session_id = hook_input.get("session_id", "unknown")
        cwd = hook_input.get("cwd", "")
        hook_event_name = hook_input.get("hook_event_name", "PreCompact")

        log(f"Processing PreCompact hook for session {session_id}", HOOK_NAME)
        log(f"Event: {hook_event_name}", HOOK_NAME)

        # Get project context
        project_info = get_project_context(cwd, HOOK_NAME)

        # Get notification characteristics for compact event
        priority, title, tags = get_compact_notification_info()

        # Create the notification message
        message = "Context window full - auto-compaction was triggered to free up space for continued conversation."

        # Play the configured audio notification first
        log(f"Playing '{COMPACT_AUDIO_FILE}' audio notification", HOOK_NAME)
        play_audio(COMPACT_AUDIO_FILE, HOOK_NAME)

        # Send the notification using shared function
        success = send_notification(
            message=message,
            title=title,
            priority=priority,
            tags=tags,
            project_info=project_info,
            session_id=session_id,
            hook_name=HOOK_NAME,
        )

        if success:
            log("PreCompact notification sent successfully", HOOK_NAME)
        else:
            log("Failed to send PreCompact notification", HOOK_NAME)

    except Exception as e:
        log(f"✗ Hook error: {e}", HOOK_NAME)
        log(f"✗ Exception type: {type(e).__name__}", HOOK_NAME)
        log(f"✗ Exception details: {str(e)}", HOOK_NAME)
        log("✗ Full traceback:", HOOK_NAME)
        import traceback

        log(traceback.format_exc(), HOOK_NAME)


if __name__ == "__main__":
    main()

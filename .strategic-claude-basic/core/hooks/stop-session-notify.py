#!/usr/bin/python3
"""
Stop hook to send development session notifications via ntfy.

This hook runs when Claude Code finishes responding and sends a summary
of the development session to a ntfy server for monitoring and tracking.
"""

import json
import os
import sys
from datetime import datetime

# Import shared notification utilities
from notifications import (
    log,
    get_project_context,
    send_notification,
    play_audio,
    PROJECT_TITLE,
    PROJECT_TAG,
)

HOOK_NAME = "stop-session-notify"


def analyze_session(transcript_path: str) -> dict:
    """Analyze the session transcript and extract key information."""
    try:
        if not os.path.exists(transcript_path):
            return {
                "activity_type": "development",
                "success": True,
                "details": "Session completed",
            }

        with open(transcript_path, "r", encoding="utf-8") as f:
            content = f.read().lower()

        # Detect primary activity
        if any(word in content for word in ["git commit", "committed"]):
            activity = "git commit"
        elif any(word in content for word in ["test", "pre-commit"]):
            activity = "testing"
        elif any(word in content for word in ["function", "class", "def"]):
            activity = "development"
        elif any(word in content for word in ["debug", "error", "fix"]):
            activity = "debugging"
        else:
            activity = "development"

        return {
            "activity_type": activity,
            "success": True,
            "details": f"{activity.title()} session completed",
        }

    except Exception as e:
        log(f"Error analyzing session: {e}", HOOK_NAME)
        return {
            "activity_type": "development",
            "success": False,
            "details": f"Session failed: {e}",
        }


def determine_notification_characteristics(session_info: dict) -> tuple[str, str, list]:
    """
    Determine notification priority, title, and tags based on session analysis.

    Returns:
        Tuple of (priority, title, tags)
    """
    activity_type = session_info["activity_type"]

    if session_info["success"] and activity_type in ["git commit", "testing"]:
        priority = "high"
        tags = ["white_check_mark", "rocket", PROJECT_TAG]
        title = f"{PROJECT_TITLE}: {activity_type.title()} Session Complete"
    elif activity_type in ["research", "documentation"]:
        priority = "default"
        tags = ["book", "lightbulb", PROJECT_TAG]
        title = f"{PROJECT_TITLE}: {activity_type.title()} Session"
    elif activity_type in ["refactoring", "debugging"]:
        priority = "default"
        tags = ["wrench", "gear", PROJECT_TAG]
        title = f"{PROJECT_TITLE}: {activity_type.title()} Session"
    else:
        priority = "default"
        tags = ["gear", "construction", PROJECT_TAG]
        title = f"{PROJECT_TITLE}: Development Session Complete"

    return priority, title, tags


def main() -> None:
    """Main hook execution."""
    try:
        # Read hook input
        hook_input = json.loads(sys.stdin.read())

        # Extract information
        session_id = hook_input.get("session_id", "unknown")
        transcript_path = hook_input.get("transcript_path", "")
        cwd = hook_input.get("cwd", "")

        log(f"Processing Stop hook for session {session_id}", HOOK_NAME)

        # Analyze session and get project context
        session_info = analyze_session(transcript_path)
        project_info = get_project_context(cwd, HOOK_NAME)

        # Determine notification characteristics
        priority, title, tags = determine_notification_characteristics(session_info)

        message = f"{session_info['details']} at {datetime.now().strftime('%H:%M')}"

        # Send notification using shared function
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
            log("Session notification sent successfully", HOOK_NAME)
            # Play toasty audio for successful session completion
            play_audio("toasty.mp3", HOOK_NAME)
        else:
            log("Failed to send session notification", HOOK_NAME)

    except Exception as e:
        log(f"✗ Hook error: {e}", HOOK_NAME)
        log(f"✗ Exception type: {type(e).__name__}", HOOK_NAME)
        log(f"✗ Exception details: {str(e)}", HOOK_NAME)
        log("✗ Full traceback:", HOOK_NAME)
        import traceback

        log(traceback.format_exc(), HOOK_NAME)


if __name__ == "__main__":
    main()

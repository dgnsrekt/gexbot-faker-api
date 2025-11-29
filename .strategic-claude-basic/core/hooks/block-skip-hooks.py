#!/usr/bin/python3
"""
PreToolUse hook to block hook bypassing and enforce quality standards.

This hook prevents bypassing code quality checks and ensures proper
development practices are followed. Sends security notifications when
bypass attempts are detected.
"""

import json
import sys

# Import shared notification utilities
from notifications import (
    PROJECT_TAG,
    PROJECT_TITLE,
    get_project_context,
    log,
    play_audio,
    send_notification,
)

HOOK_NAME = "block-skip-hooks"

# Configuration - set to False to disable hook bypass protection
ENABLE_HOOK_BYPASS_PROTECTION = True

# Generic quality fix suggestions (customize for your project)
QUALITY_FIX_COMMANDS = [
    "• Fix formatting issues",
    "• Address linting warnings",
    "• Resolve test failures",
    "• Fix pre-commit hook violations",
]


def send_security_alert(
    violation_type: str, command: str, session_id: str, cwd: str, details: str
) -> None:
    """
    Send a high-priority security notification about hook bypass attempts.

    Args:
        violation_type: Type of security violation (e.g., "SKIP bypass", "no-verify bypass")
        command: The blocked command
        session_id: Claude Code session ID
        cwd: Current working directory
        details: Additional details about the violation
    """
    try:
        project_info = get_project_context(cwd, HOOK_NAME)

        message = f"""Security Alert: Hook Bypass Attempt

**Violation**: {violation_type}
**Command**: {command}
**Details**: {details}

This attempt was BLOCKED to maintain code quality standards.
All hook bypass attempts are logged and monitored."""

        success = send_notification(
            message=message,
            title=f"🚨 {PROJECT_TITLE}: Security Alert",
            priority="high",
            tags=["rotating_light", "warning", "security", PROJECT_TAG],
            project_info=project_info,
            session_id=session_id,
            hook_name=HOOK_NAME,
        )

        if success:
            log("Security alert notification sent", HOOK_NAME)
            # Play gutter-trash audio for security violations
            play_audio("gutter-trash.mp3", HOOK_NAME)
        else:
            log("Failed to send security alert notification", HOOK_NAME)

    except Exception as e:
        log(f"Error sending security alert: {e}", HOOK_NAME)


def main() -> None:
    """Check and block inappropriate git and development commands."""
    try:
        # Check if hook bypass protection is enabled
        if not ENABLE_HOOK_BYPASS_PROTECTION:
            sys.stdout.write(json.dumps({"decision": "approve"}) + "\n")
            return

        # Read the hook input from stdin
        hook_input = json.loads(sys.stdin.read())

        # Extract tool information
        tool_input = hook_input.get("tool_input", {})
        session_id = hook_input.get("session_id", "unknown")
        cwd = hook_input.get("cwd", "")

        # Get the command being executed
        command = tool_input.get("command", "")

        # Block git hook bypasses
        if "git commit" in command and (
            "SKIP=" in command
            or " -n" in command
            or "--no-verify" in command
            or "core.hooksPath=/dev/null" in command
            or "GIT_HOOKS_PATH=/dev/null" in command
        ):
            fix_commands = "\n".join(QUALITY_FIX_COMMANDS)
            error_message = (
                "🚫 Blocked: Bypassing Git hooks is not allowed.\n\n"
                f"Fix the actual code issues:\n"
                f"{fix_commands}\n\n"
                f"Quality checks ensure {PROJECT_TITLE} excellence."
            )

            send_security_alert(
                violation_type="Git hook bypass attempt",
                command=command,
                session_id=session_id,
                cwd=cwd,
                details="Attempted to bypass Git quality hooks",
            )

            sys.stdout.write(
                json.dumps({"decision": "block", "message": error_message}) + "\n"
            )
            return

        # Block git commit amend and no-edit flags
        if "git commit" in command and ("--amend" in command or "--no-edit" in command):
            error_message = (
                "🚫 Blocked: Git commit amendments without proper review are not allowed.\n\n"
                "Instead of amending:\n"
                "• Create a new commit with proper description\n"
                "• Use interactive rebase if history needs modification\n"
                "• Ensure all changes are properly reviewed\n\n"
                f"Transparent commit history maintains {PROJECT_TITLE} integrity."
            )

            send_security_alert(
                violation_type="Git commit amendment attempt",
                command=command,
                session_id=session_id,
                cwd=cwd,
                details="Attempted to amend commit without proper editing/review",
            )

            sys.stdout.write(
                json.dumps({"decision": "block", "message": error_message}) + "\n"
            )
            return

        # Allow all other commands
        sys.stdout.write(json.dumps({"decision": "approve"}) + "\n")

    except Exception as e:
        # On error, allow the command but log the issue
        log(f"Hook error: {e}", HOOK_NAME)
        log(f"Exception type: {type(e).__name__}", HOOK_NAME)
        sys.stdout.write(json.dumps({"decision": "approve"}) + "\n")


if __name__ == "__main__":
    main()

#!/usr/bin/python3
"""
PreToolUse hook to block modifications to quality configuration files.

This hook prevents modifications to linting and pre-commit configuration files
to maintain consistent code quality standards across the project.
"""

import json
import sys

# Import shared notification utilities
from notifications import (
    log,
    get_project_context,
    send_notification,
    play_audio,
    PROJECT_TITLE,
    PROJECT_TAG,
)

HOOK_NAME = "block-config-writes"

# Configuration - set to False to disable config file protection
ENABLE_CONFIG_PROTECTION = True

# Protected configuration files - modify this list for your project needs
PROTECTED_CONFIG_FILES = [
    ".golangci.yml",  # Go linting config
    ".pre-commit-config.yaml",  # Pre-commit hooks config
    # Add more protected files as needed
]


def send_security_alert(
    violation_type: str, file_path: str, session_id: str, cwd: str, details: str
) -> None:
    """
    Send a high-priority security notification about config modification attempts.

    Args:
        violation_type: Type of security violation (e.g., "Config modification")
        file_path: The file being modified
        session_id: Claude Code session ID
        cwd: Current working directory
        details: Additional details about the violation
    """
    try:
        project_info = get_project_context(cwd, HOOK_NAME)

        message = f"""Security Alert: Config Modification Attempt

**Violation**: {violation_type}
**File**: {file_path}
**Details**: {details}

This attempt was BLOCKED to maintain code quality standards.
Fix the code, not the linting rules."""

        success = send_notification(
            message=message,
            title=f"🚨 {PROJECT_TITLE}: Config Protection",
            priority="high",
            tags=["rotating_light", "warning", "config", PROJECT_TAG],
            project_info=project_info,
            session_id=session_id,
            hook_name=HOOK_NAME,
        )

        if success:
            log("Config protection alert notification sent", HOOK_NAME)
            # Play that-was-pathetic audio for security violations
            play_audio("that-was-pathetic.mp3", HOOK_NAME)
        else:
            log("Failed to send config protection alert notification", HOOK_NAME)

    except Exception as e:
        log(f"Error sending config protection alert: {e}", HOOK_NAME)


def main() -> None:
    """Check and block modifications to quality configuration files."""
    try:
        # Check if config protection is enabled
        if not ENABLE_CONFIG_PROTECTION:
            sys.stdout.write(json.dumps({"decision": "approve"}) + "\n")
            return

        # Read the hook input from stdin
        hook_input = json.loads(sys.stdin.read())

        # Extract tool information
        tool_input = hook_input.get("tool_input", {})
        session_id = hook_input.get("session_id", "unknown")
        cwd = hook_input.get("cwd", "")

        # Get the file path being written to
        file_path = tool_input.get("file_path", "")

        # Use the configured list of protected configuration files
        protected_configs = PROTECTED_CONFIG_FILES

        # Check if any protected config file is being modified
        for config_file in protected_configs:
            if config_file in file_path:
                error_message = (
                    f"🚫 Blocked: Quality config modification not allowed.\n\n"
                    f"File: {config_file}\n"
                    f"Fix the code, not the linting rules.\n\n"
                    f"Quality standards ensure {PROJECT_TITLE} excellence."
                )

                send_security_alert(
                    violation_type="Config modification attempt",
                    file_path=file_path,
                    session_id=session_id,
                    cwd=cwd,
                    details=f"Attempted to modify {config_file}",
                )

                sys.stdout.write(
                    json.dumps({"decision": "block", "message": error_message}) + "\n"
                )
                return

        # Allow all other file modifications
        sys.stdout.write(json.dumps({"decision": "approve"}) + "\n")

    except Exception as e:
        # On error, allow the command but log the issue
        log(f"Hook error: {e}", HOOK_NAME)
        log(f"Exception type: {type(e).__name__}", HOOK_NAME)
        sys.stdout.write(json.dumps({"decision": "approve"}) + "\n")


if __name__ == "__main__":
    main()

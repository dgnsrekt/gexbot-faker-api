---
description: "Review the most recent committed changes using the codex-review agent"
argument-hint: [optional_commit_ref]
allowed-tools: Task, Bash(git:*)
model: claude-opus-4-5
---

You are tasked with reviewing code changes using the specialized codex-review agent. This command analyzes the most recent committed changes (or a specific commit if provided) and provides comprehensive code review feedback.

**Commit reference (optional):** $1

## Process

1. **Determine commit to review:**

   - If a commit reference is provided as $1, use that specific commit
   - Otherwise, review the most recent commit (HEAD)

2. **Launch codex-review agent:**

   - Use the Task tool with subagent_type "codex-review"
   - Instruct the agent to analyze the specified commit changes
   - The agent will provide prioritized findings, bug detection, and actionable recommendations

3. **Present results:**
   - Display the comprehensive code review report from the codex-review agent
   - Highlight any P0/P1 issues that require immediate attention
   - Summarize key recommendations for the user

## Command Implementation

This command provides a quick way to get professional code review feedback on recent changes without manually invoking the codex-review agent.

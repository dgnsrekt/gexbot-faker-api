---
date: [Current date and time with timezone in ISO format]
git_commit: [Current commit hash]
branch: [Current branch name]
repository: [Repository name]
plan_reference: "[Related plan document filename]"
phase: "[Phase X.Y: Phase Name if applicable]"
status: [complete | blocked | partial]
completion_rate: "[X% complete]"
critical_issues: [Number of critical blocking issues]
last_updated: [Current date in YYYY-MM-DD format]
---

# SUMMARY*[TASK_NAME]*[YYYYMMDD]

## Overview

[2-3 sentence summary of what was attempted and current status - focus on what's broken or incomplete]

## Outstanding Issues & Incomplete Work

### Critical Issues

Issues that block functionality or prevent progress:

- 🔴 **[Issue Title]** (CRITICAL) - [Brief description]
  - **Impact**: [What this breaks or prevents]
  - **Root Cause**: [Why this happened]
  - **Resolution**: [What needs to be done to fix it]
  - **Estimated Time**: [Time needed to resolve]

### Incomplete Tasks

Tasks from the plan that were not completed:

- 🔧 **[Task Name]** - [What was missed]
  - **Reason**: [Why it wasn't completed]
  - **Impact**: [What functionality is missing]
  - **Next Step**: [What needs to happen]

### Hidden TODOs & Technical Debt

Code TODOs and shortcuts discovered during review:

- 🧩 **[File:line]** - [TODO comment or technical shortcut]
  - **Impact**: [What this affects]
  - **Refactoring Needed**: [What should be done]

### Discovered Problems

Issues found during implementation that weren't in the original plan:

- 🟡 **[Problem Title]** - [Description]
  - **Context**: [How this was discovered]
  - **Priority**: [HIGH/MEDIUM/LOW] - [Why this priority]
  - **Effort**: [Time needed to address]

## Brief Implementation Summary

### What Was Implemented

- [Concise bullet point of major completion]
- [Another key deliverable that works]

### Files Modified/Created

- `path/to/file.ext` - [Brief description of changes]
- `another/file.js` - [What was added/modified]

## Problems That Need Immediate Attention

1. **[Problem 1]** - [Why this needs immediate focus]
2. **[Problem 2]** - [Another urgent issue]

## References

- **Source Plan**: `[path/to/plan.md]`
- **Related Research**: `[path/to/research.md]` (if applicable)
- **Modified Files**: [List of key files changed]

---

**Implementation Status**: [🔴 BLOCKED | 🟡 PARTIAL | ✅ COMPLETE] - [One sentence status summary focused on what still needs work]

---
date: [Current date and time with timezone in ISO format]
git_commit: [Current commit hash]
branch: [Current branch name]
repository: [Repository name]
severity: [critical | high | medium | low]
status: [open | in_progress | resolved]
affected_files: [List of key files affected]
last_updated: [Current date in YYYY-MM-DD format]
---

# ISSUE_[NNN]_[BRIEF_DESCRIPTION]

## Problem

[1-2 sentence description of what's broken or not working as expected]

**Affected Components:**
- `path/to/file.ext:line` - [Brief description]
- `another/file.js:line` - [Brief description]

## Impact

**Severity:** [Critical/High/Medium/Low]
**Why this matters:** [Brief explanation of impact on functionality or users]

## Solution

**Recommended:** Solution [1/2/3]

### Solution 1: [Brief Solution Name] ⭐ **(Recommended)**

[Brief description of the recommended approach]

```[language]
// Code example showing the fix
```

**Why recommended:** [1 sentence explaining why this is the best approach]

### Solution 2: [Alternative Solution Name]

[Brief description of alternative approach]

**Trade-offs:** [1 sentence about pros/cons vs recommended solution]

### Solution 3: [Another Alternative] *(if applicable)*

[Brief description of third option]

**Trade-offs:** [1 sentence about when this might be appropriate]

## Verification

**Quick Test:**
```bash
# Command to verify the fix works
make test
```

**Expected Result:** [What should happen when the fix is working]

## References

- **Related Plan:** `[path/to/plan.md]` (if applicable)
- **Source Files:** [List of key files that need changes]

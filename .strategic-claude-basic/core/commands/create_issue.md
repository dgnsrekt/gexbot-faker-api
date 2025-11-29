---
description: "Create issue documentation for problems found during development work"
argument-hint: <subject_or_summary_file>
allowed-tools: Read(./**), Write(./.strategic-claude-basic/issues/**), Bash(git:*, date:*, grep:*), Glob
model: claude-opus-4-5
---

You are tasked with creating brief, solution-focused issue documentation for problems discovered during development work. You should be fast, focused, and avoid extensive research - document what you know from context.

**Subject/File provided:** $1

## Initial Response

When this command is invoked:

1. **Check if subject/file was provided**:

   - If subject/file was provided as parameter, proceed with analysis
   - If no subject provided, respond with:

   ```
   I'll help you create an issue document for a problem you've discovered.

   Please provide:
   1. A brief description of the issue, or
   2. A summary file path to extract issues from

   Examples:
   - `/create_issue authentication failing in production`
   - `/create_issue @.strategic-claude-basic/summary/SUMMARY_0001_07-09-2025_sat_user-auth.md`

   I'll document the issue quickly without extensive solution research.
   ```

   Then wait for the user's input.

## Process Steps

### Step 1: Input Analysis

1. **Determine input type**:

   **If it's a file path (starts with @. or contains .md)**:

   - Read the specified file completely
   - Look for sections about problems, issues, or incomplete work
   - Prepare to create multiple issues if needed

   **If it's a description**:

   - Use the provided subject as the issue description
   - Look at conversation context for additional details
   - Prepare to create a single issue

2. **Present scope**:

   **For file input**:

   ```
   Reading summary file: [filename]

   Found [N] issues to document:
   - [Brief issue 1]
   - [Brief issue 2]
   - [Brief issue 3]

   I'll create individual issue documents for each problem found.
   Proceeding with issue creation...
   ```

   **For subject input**:

   ```
   Creating issue documentation for: [subject]

   I'll document this issue based on the current context and any known details.
   Proceeding with issue creation...
   ```

### Step 2: Issue Extraction and Analysis

**CRITICAL**: Do NOT spawn research agents or extensive analysis. Only use information already available.

1. **For summary files**:

   - Extract all items from "Outstanding Issues & Incomplete Work" sections
   - Extract items from "Critical Issues" sections
   - Extract items from "Discovered Problems" sections
   - Note any solutions already mentioned in the summary

2. **For single issue subjects**:

   - Use conversation context to understand the problem
   - Look for any error messages or failures mentioned
   - Check for any solutions already discussed

3. **Gather basic context**:

   - Affected files mentioned in conversation or summary
   - Severity based on problem description
   - Any known workarounds or solutions from context

### Step 3: Issue Documentation

1. **Gather metadata for issue documents**:

   ```bash
   # Get current date/time with timezone
   date --iso-8601=seconds

   # Get current git commit hash
   git rev-parse HEAD

   # Get current branch name
   git branch --show-current

   # Get repository name
   basename -s .git $(git config --get remote.origin.url)

   # Get formatted date for filename
   date '+%d-%m-%Y-%a' | tr '[:upper:]' '[:lower:]'
   ```

2. **For each issue to create**:

   - Generate filename using pattern: `ISSUE_NNNN_DD-MM-YYYY_day_brief-subject.md`
   - Use template: `@.strategic-claude-basic/templates/commands/issue.template.md`
   - Replace ALL bracketed placeholders with actual information
   - Follow naming convention from: `@.strategic-claude-basic/issues/CLAUDE.md`

3. **Solution handling**:

   **If solution is known from context**:

   - Document it in the Solution section
   - Mark it as "Recommended"
   - Include any code examples mentioned

   **If solution is unknown**:

   - Use placeholder: "Solution pending - requires investigation"
   - Mark severity appropriately
   - Document what's known about the problem

4. **Write documents**:

   - Write document to: `.strategic-claude-basic/issues/[filename]`
   - Update the `@.strategic-claude-basic/issues/CLAUDE.md` file with new document entries

### Step 4: Present Results

1. **For single issue**:

   ```
   Created issue document:
   `.strategic-claude-basic/issues/ISSUE_NNNN_DD-MM-YYYY_day_brief-subject.md`

   Issue: [Brief description]
   Severity: [Level]
   Solution: [Known/Pending]

   The issue is documented and ready for resolution.
   ```

2. **For multiple issues from summary**:

   ```
   Created [N] issue documents from summary:

   - ISSUE_NNNN_DD-MM-YYYY_day_issue1.md - [Brief description]
   - ISSUE_NNNN_DD-MM-YYYY_day_issue2.md - [Brief description]
   - ISSUE_NNNN_DD-MM-YYYY_day_issue3.md - [Brief description]

   Issues extracted: [N] critical, [N] high, [N] medium priority
   Solutions known: [N], Solutions pending: [N]

   All issues are documented and ready for resolution.
   ```

## Important Guidelines

1. **Be Fast**:

   - No extensive research or agent spawning
   - Use only information from context and provided files
   - Document what you know, mark unknowns as pending

2. **Be Focused**:

   - Create one issue per distinct problem
   - Keep problem descriptions brief but clear
   - Include solutions only if already known

3. **Be Consistent**:

   - Follow the issue template exactly
   - Use proper naming conventions
   - Update CLAUDE.md tracking

4. **Be Practical**:

   - Mark severity based on impact described
   - Include affected files when mentioned
   - Provide verification steps when obvious

## Common Scenarios

### Scenario 1: Single Issue from Conversation

When user describes a specific problem:

- Extract problem description from context
- Look for error messages or symptoms mentioned
- Document any solutions already discussed
- Create single issue document

### Scenario 2: Multiple Issues from Summary

When user provides a summary file:

- Read summary completely
- Extract all distinct problems mentioned
- Create separate issue for each problem
- Batch create all issue documents

### Scenario 3: Issue Without Known Solution

When problem is documented but solution is unknown:

- Document the problem clearly
- Mark solution as "pending investigation"
- Include what troubleshooting was already attempted
- Set appropriate severity level

## Success Criteria

A good issue document should:

- **Be actionable**: Clear problem statement and next steps
- **Be specific**: Reference exact files, errors, or symptoms
- **Be appropriate**: Correct severity and priority
- **Be complete**: All known information captured
- **Be ready**: Prepared for immediate resolution work

The goal is rapid documentation, not comprehensive analysis. Capture what you know and move on.

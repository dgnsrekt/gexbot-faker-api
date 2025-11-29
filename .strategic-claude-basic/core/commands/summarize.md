---
description: "Generate a problem-focused summary of the current implementation work"
argument-hint: [optional_plan_reference]
allowed-tools: Read(./**), Write(./.strategic-claude-basic/summary/**), Bash(git:*, date:*, grep:*), Glob
model: claude-opus-4-5
---

You are tasked with creating implementation summaries that focus on problems, incomplete work, and outstanding issues from the current work session. You should be problem-focused, session-scoped, and work only with uncommitted changes and current context.

**Plan reference (optional):** $1

## Initial Response

When this command is invoked:

1. **Check for uncommitted work first**:

   ```bash
   git status --porcelain
   ```

2. **If uncommitted changes exist**, proceed immediately to context analysis

3. **If there are no uncommitted changes, respond with**:

   ```
   I don't see any uncommitted changes in your repository.

   A summary documents the problems and incomplete work from your current implementation session.

   Please:
   1. Complete some implementation work, or
   2. Specify what you'd like me to summarize if you've already committed your changes

   Usage: `/summarize` - Auto-detect current work
   Usage: `/summarize PLAN_0001_...` - Summarize work for specific plan
   ```

## Process Steps

### Step 1: Current Session Context Analysis

1. **Check uncommitted changes**:

   ```bash
   # Check what files were modified in current session
   git status --porcelain

   # Get detailed changes
   git diff
   git diff --cached
   ```

2. **Analyze the current conversation context**:

   - Review what work was just completed in this chat session
   - Identify if a specific plan was being followed
   - Note any tests that were run and their results
   - Track any problems encountered during implementation

3. **Search for associated plan document**:

   - If plan reference was provided as $1, use that
   - Otherwise, look in `.strategic-claude-basic/plan/` for recent plans
   - Check conversation context for plan references
   - Look at file changes to infer what feature was being worked on

4. **Determine summary scope**:

   ```
   Found uncommitted changes in [N] files.

   Based on the session context, I can see you were working on: [brief description]

   [If plan found]: This appears to be implementing Plan: [plan filename]
   [If no plan]: This appears to be standalone implementation work

   I'll create a summary focusing on problems and incomplete work from this session.
   Proceeding with summary generation...
   ```

### Step 2: Session-Scoped Problem Analysis

**CRITICAL**: Only analyze problems from the CURRENT WORK SESSION. Do not search the entire codebase.

1. **Analyze uncommitted changes for problems**:

   - Search for TODO comments added in current changes
   - Look for FIXME, HACK, or similar markers in new code
   - Check for incomplete implementations in modified files
   - Identify any commented-out code or placeholder functions

2. **Review test results from this session**:

   - If tests were run in the conversation, check their results
   - Note any failing tests that were discovered
   - Identify tests that are missing or incomplete

3. **Cross-reference with plan (if applicable)**:

   - **IMPORTANT**: Only if a plan was being followed
   - Compare what was planned vs. what was actually completed
   - Identify tasks from the plan that remain incomplete
   - Note any deviations from the original plan

4. **Identify technical debt introduced**:

   - Look for shortcuts taken in the current implementation
   - Note any error handling that was deferred
   - Identify areas where code quality was compromised for speed

### Step 3: Problem Documentation

**Focus Areas** (from current session only):

1. **Critical Issues**:

   - Blocking problems that prevent functionality
   - Test failures discovered during implementation
   - Integration issues encountered

2. **Incomplete Tasks**:

   - Planned work that wasn't completed
   - TODOs left in the code
   - Features that are partially implemented

3. **Technical Shortcuts**:

   - Quick fixes that need proper implementation
   - Error handling that was deferred
   - Code that needs refactoring

4. **Discovered Problems**:
   - Issues found during implementation
   - Edge cases that weren't anticipated
   - Dependencies or constraints discovered

### Step 4: Document Generation

1. **Gather metadata for summary document**:

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

2. **Determine naming convention**:

   **If connected to a PLAN document**:

   - Use EXACT same NNNN number as the plan
   - Use EXACT same subject as the plan
   - Format: `SUMMARY_PLAN_NNNN_DD-MM-YYYY_day_subject.md`

   **If connected to a TEST document**:

   - Use EXACT same NNNN number as the test plan
   - Use EXACT same subject as the test plan
   - Format: `SUMMARY_TEST_NNNN_DD-MM-YYYY_day_subject.md`

   **If no plan connection**:

   - Omit NNNN number
   - Use descriptive subject based on work completed
   - Format: `SUMMARY_DD-MM-YYYY_day_subject.md`

3. **Generate summary document**:

   - Use template: @.strategic-claude-basic/templates/commands/summary.template.md
   - Replace ALL bracketed placeholders with actual metadata
   - Follow naming convention from: @.strategic-claude-basic/summary/CLAUDE.md
   - Write document to: .strategic-claude-basic/summary/[filename]
   - Update the @.strategic-claude-basic/summary/CLAUDE.md file with new document entry

### Step 5: Present Results

1. **Present summary location and key findings**:

   ```
   I've created an implementation summary at:
   `.strategic-claude-basic/summary/[filename].md`

   Key problems identified from this session:
   - [Critical issue 1]
   - [Incomplete task 1]
   - [Technical debt introduced]

   [If plan connected]: This summarizes the work on [plan name]
   [If tests failed]: [Number] tests are currently failing
   [If TODOs added]: [Number] TODOs were left in the code

   Review the summary before committing your changes.
   ```

## Important Guidelines

1. **Session-Scoped Only**:

   - Only analyze work from the current session
   - Do not search for unrelated codebase issues
   - Focus on uncommitted changes and conversation context
   - Do not audit the entire project

2. **Problem-Focused**:

   - Prioritize what's broken or incomplete
   - Don't celebrate achievements (that's not the purpose)
   - Highlight issues that need immediate attention
   - Focus on what requires follow-up work

3. **Pre-Commit Timing**:

   - Designed to run before committing changes
   - Captures problems while they're fresh in context
   - Documents issues for future reference
   - Helps track what needs cleanup

4. **Context-Driven**:

   - Use conversation history to understand what was worked on
   - Reference specific files and changes made
   - Connect problems to the work that was attempted
   - Maintain traceability to the original plan if applicable

5. **Follow Standards**:

   - Use the summary template consistently
   - Match plan naming when connected to a plan (include PLAN or TEST prefix)
   - Update CLAUDE.md with new document entries
   - Include proper metadata and references

## Common Scenarios

### Scenario 1: Plan Implementation

When working from an existing PLAN or TEST document:

- Match the document's NNNN number and subject exactly
- Include PLAN or TEST prefix in the summary filename
- Compare actual work done vs. planned tasks
- Identify which plan tasks remain incomplete
- Note any deviations or discoveries made during implementation

### Scenario 2: Bug Fixes or Maintenance

When doing standalone work:

- Use date-based naming without NNNN
- Focus on what was attempted vs. what was achieved
- Document any new issues discovered
- Note any shortcuts taken that need follow-up

### Scenario 3: Failed Implementation

When work doesn't complete successfully:

- Document why the approach failed
- List blockers that prevented completion
- Identify what needs to change for success
- Preserve lessons learned for future attempts

## Example Usage

```
User: /summarize
Assistant: [Checks git status, finds changes, analyzes conversation]
Assistant: Found uncommitted changes in 5 files. You were implementing user authentication from PLAN_0001_06-09-2025_fri_user-auth-system.md. Creating summary...

User: /summarize PLAN_0002_07-09-2025_sat_api-refactoring
Assistant: [Analyzes work done for that specific plan]
```

## Success Criteria

A good summary should:

- **Focus on problems**: What's broken, incomplete, or needs attention
- **Be actionable**: Clear next steps for addressing issues
- **Be specific**: Reference exact files, line numbers, and error messages
- **Be scoped**: Only cover the current session's work
- **Be connected**: Link back to the plan if one was being followed
- **Be timely**: Created before committing to preserve context

The summary is NOT a celebration of achievements - it's a problem report and action plan for addressing incomplete work.

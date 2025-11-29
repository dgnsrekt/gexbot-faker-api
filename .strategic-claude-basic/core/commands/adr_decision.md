---
description: "Manage ADR status transitions (accept, reject, deprecate, supersede)"
argument-hint: <adr_file_path> <--accept|--reject|--deprecate|--supersede>
allowed-tools: Read(./**), Edit(./.strategic-claude-basic/decisions/**), Bash(git:*, date:*), Glob
model: claude-opus-4-5
---

You are tasked with managing Architecture Decision Record (ADR) status transitions. You will update ADR documents through their lifecycle states: proposed → accepted/rejected, accepted → deprecated/superseded.

**ADR file path and status flag provided:** $1

## Initial Response

When this command is invoked:

1. **Check if ADR file path and flag were provided**:

   - If both ADR file path and status flag were provided, proceed with validation
   - If missing parameters, respond with:

   ```
   I'll help you update an ADR's status through its lifecycle.

   Usage: `/adr_decision <adr_file_path> <--flag>`

   Available flags:
   - `--accept`: Change proposed ADR to accepted
   - `--reject`: Change proposed ADR to rejected
   - `--deprecate`: Change accepted ADR to deprecated
   - `--supersede`: Change accepted/deprecated ADR to superseded

   Examples:
   - `/adr_decision @.strategic-claude-basic/decisions/ADR_0001_17-09-2025_wed_backwards-compatibility-policy.md --accept`
   - `/adr_decision ADR_0001_17-09-2025_wed_backwards-compatibility-policy.md --supersede`

   Provide the ADR file path and the status transition flag.
   ```

   Then wait for the user's input.

## Process Steps

### Step 1: Input Validation and File Analysis

1. **Parse command arguments**:

   - Extract ADR file path from first argument
   - Extract status flag from second argument (--accept, --reject, --deprecate, --supersede)
   - Validate flag is one of the four allowed options

2. **Locate and read ADR file**:

   ```bash
   # If path doesn't start with ., add the decisions directory prefix
   if [[ "$adr_path" != ./* ]]; then
     adr_path=".strategic-claude-basic/decisions/$adr_path"
   fi
   ```

3. **Validate ADR file exists and read content**:

   - Check if file exists and is readable
   - Read complete ADR document
   - Extract current status from frontmatter
   - Extract decision number for tracking

4. **Present current state**:

   ```
   ADR Analysis:
   File: [filename]
   Decision: ADR-NNNN - [title]
   Current Status: [current_status]
   Requested Transition: [current_status] → [new_status]

   [Validation result: Valid/Invalid transition]
   Proceeding with status update...
   ```

### Step 2: Status Transition Validation

1. **Validate transition rules**:

   **--accept flag**:

   - Only allowed from "proposed" → "accepted"
   - Reject if current status is not "proposed"

   **--reject flag**:

   - Only allowed from "proposed" → "rejected"
   - Reject if current status is not "proposed"

   **--deprecate flag**:

   - Only allowed from "accepted" → "deprecated"
   - Reject if current status is not "accepted"

   **--supersede flag**:

   - Allowed from "accepted" → "superseded"
   - Allowed from "deprecated" → "superseded"
   - Require new ADR reference for superseding

2. **Handle superseding workflow**:

   **If --supersede flag is used**:

   ```
   This ADR will be superseded by a new decision.

   Please provide the ADR number that supersedes this decision:
   Example: ADR-0002

   I'll update both ADRs with proper cross-references.
   ```

   Wait for user input if superseding.

3. **Gather transition metadata**:

   ```bash
   # Get current date and git info for transition tracking
   date --iso-8601=seconds
   git rev-parse HEAD
   date '+%Y-%m-%d'
   ```

### Step 3: ADR Document Updates

1. **Update ADR frontmatter**:

   - Change `status` field to new status
   - Update `last_updated` field to current date
   - For superseding: add `superseded_by` field with new ADR reference
   - For superseding: update new ADR with `supersedes` field

2. **Update ADR body sections**:

   - Update "## Status" section with new status
   - For superseding: add superseding reference text
   - Add transition note with date and reasoning

3. **Handle superseding cross-references**:

   **For the superseded ADR**:

   - Add "Superseded by ADR-XXXX on [date]" to Status section
   - Update superseded_by field in frontmatter

   **For the superseding ADR** (if it exists):

   - Add current ADR to supersedes array in frontmatter
   - Add "Supersedes ADR-XXXX" to Status section

### Step 4: Update Tracking and Documentation

1. **Update CLAUDE.md tracking file**:

   - Read current CLAUDE.md file
   - Update the document status in "Existing Documents" section
   - Add status change note if significant

2. **Git commit recommendation**:

   ```
   Status transition completed successfully.

   Recommended commit message:
   "Update ADR-NNNN status: [old_status] → [new_status]

   - [Brief reason for transition]
   - Updated decision tracking
   [If superseding: - Cross-referenced with ADR-XXXX]

   🤖 Generated with [Claude Code](https://claude.ai/code)

   Co-Authored-By: Claude <noreply@anthropic.com>"
   ```

### Step 5: Present Results

1. **For successful transitions**:

   ```
   ✅ ADR Status Updated Successfully

   ADR: ADR-NNNN - [title]
   Transition: [old_status] → [new_status]
   Updated: [current_date]

   Files Modified:
   - [adr_filename] - Status updated
   - CLAUDE.md - Tracking updated
   [If superseding: - [superseding_adr_filename] - Cross-reference added]

   The ADR is now in "[new_status]" status and ready for use.
   ```

2. **For invalid transitions**:

   ```
   ❌ Invalid Status Transition

   ADR: ADR-NNNN - [title]
   Current Status: [current_status]
   Requested Transition: → [requested_status]

   Valid transitions from "[current_status]":
   [List of valid transition options]

   Please use the correct flag for a valid transition.
   ```

## Important Guidelines

1. **Status Validation**:

   - Strictly enforce transition rules
   - Never allow invalid status changes
   - Provide clear error messages for invalid requests

2. **File Safety**:

   - Always backup original status before changes
   - Use atomic file operations where possible
   - Validate file integrity after updates

3. **Cross-Reference Management**:

   - For superseding, always update both ADRs
   - Maintain bidirectional references
   - Validate superseding ADR exists before updating

4. **Tracking Consistency**:

   - Always update CLAUDE.md when status changes
   - Maintain sequential numbering integrity
   - Keep decision history accurate

## Status Transition Rules Summary

| Current Status | Valid Transitions        |
| -------------- | ------------------------ |
| proposed       | --accept, --reject       |
| accepted       | --deprecate, --supersede |
| rejected       | None (final state)       |
| deprecated     | --supersede              |
| superseded     | None (final state)       |

## Common Scenarios

### Scenario 1: Accepting a Proposed ADR

User provides proposed ADR with --accept flag:

- Validate current status is "proposed"
- Update to "accepted"
- Add acceptance date and commit info
- Update tracking

### Scenario 2: Superseding an Accepted ADR

User provides accepted ADR with --supersede flag:

- Prompt for superseding ADR number
- Update current ADR to "superseded"
- Add superseded_by reference
- Update superseding ADR with supersedes reference
- Update tracking for both ADRs

### Scenario 3: Invalid Transition Attempt

User tries to accept an already accepted ADR:

- Detect invalid transition
- Explain current status and valid options
- Provide guidance on correct flag usage

## Success Criteria

A successful ADR status transition should:

- **Be valid**: Follow established transition rules
- **Be complete**: Update all relevant files and references
- **Be tracked**: Maintain accurate decision history
- **Be documented**: Provide clear transition reasoning
- **Be atomic**: Complete all updates or fail cleanly

The goal is reliable, traceable ADR lifecycle management with proper status transitions and cross-referencing.

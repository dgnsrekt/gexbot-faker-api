---
description: "Read and execute a plan document from .strategic-claude-basic/plan/"
argument-hint: <plan_file> [--with-codex]
model: claude-opus-4-5
---

# Implement Plan

You are tasked with implementing an approved technical plan from `.strategic-claude-basic/plan/`. These plans contain phases with specific changes and success criteria.

**Plan file provided:** $1

## Optional Pair Programming Navigator (`--with-codex`)

When the `--with-codex` flag is present, enable Codex pair-programming mode using the codex-navigator subagent:

### Navigator Setup and Context Sharing

1. **Launch Navigator**: Use the Task tool to start the codex-navigator subagent:

   ```
   Use Task tool with subagent_type: "codex-navigator"
   Prompt: "I'm implementing [plan name] and need strategic navigation guidance.

   Plan Context:
   - Goal: [primary objective from plan]
   - Current Phase: [phase being worked on]
   - Key Constraints: [any critical limitations or requirements]
   - Files Involved: [main files mentioned in plan]

   Please provide strategic navigation for this implementation, focusing on architecture, patterns, and quality oversight while I handle the tactical implementation."
   ```

2. **Navigator Availability Handling**: If the subagent reports that Codex MCP is unavailable, continue in solo mode:
   ```
   ❌ Codex MCP Navigator unavailable — continuing in solo driver mode.
   Please configure the Codex MCP server and rerun with `--with-codex` when ready.
   ```

### Driver-Navigator Collaboration Pattern

3. **Structured Communication Cycle**:

   - **Before significant actions**: Consult Navigator about approach, risks, and best practices
   - **During implementation**: Update Navigator on progress and any challenges encountered
   - **After completing sections**: Request feedback and guidance for next steps
   - **At phase boundaries**: Confirm completion criteria and validate phase transitions

4. **Driver Self-Reflection Protocol**:

   - After receiving Navigator guidance, restate the approach in your own words
   - Evaluate implementation risks and confirm alignment with plan success criteria
   - Proactively surface uncertainties or validation needs
   - Sanity-check that Navigator's roadmap matches plan phases and checkbox items

5. **Quality Checkpoints**: Use Navigator for:
   - Architecture review and pattern consistency
   - Code quality and best practices verification
   - Security and performance considerations
   - Testing strategy validation

### Session Completion Validation

6. **Final Navigator Review**: Before marking plan items complete, confirm with Navigator that:
   - All strategic objectives have been met
   - No architectural concerns remain unaddressed
   - Implementation follows established patterns and standards
   - No quality issues or technical debt was introduced

## CRITICAL: Checkbox-Only Updates

**NEVER modify the plan text**. The ONLY changes allowed are:

- Changing `- [ ]` to `- [x]` when a task is complete
- NO summaries, NO additional details, NO implementation notes
- The plan document is a CONTRACT - preserve it exactly as written

**Example of correct checkbox update:**

```
OLD: "- [ ] Task description"
NEW: "- [x] Task description"
```

**DO NOT add comments, notes, or ANY other text to the plan file.**

## Getting Started

**If no plan path provided:**

```
I'll help you implement a plan from .strategic-claude-basic/plan/

Please provide the plan filename you'd like me to execute.
Example: `/read_and_execute_plan PLAN_0001_07-09-2025_sat_user-authentication.md`
```

**When given a plan path:**

1. Determine the actual plan file argument (ignore the optional `--with-codex` flag when present), then read the plan completely and check for any existing summary for the plan in `.strategic-claude-basic/summary/`
2. Read all files mentioned in the plan
3. If a corresponding summary exists read that summary and note the progress that may have been completed already
4. **Read files fully** - never use limit/offset parameters, you need complete context
5. Think deeply about how the pieces fit together
6. Create a todo list to track your progress
7. Start implementing if you understand what needs to be done

## Implementation Philosophy

Plans are carefully designed, but reality can be messy. Your job is to:

- Follow the plan's intent while adapting to what you find
- Implement each phase fully before moving to the next
- Verify your work makes sense in the broader codebase context
- Update ONLY checkboxes in the plan as you complete sections (change `- [ ]` to `- [x]`)

When things don't match the plan exactly, think about why and communicate clearly. The plan is your guide, but your judgment matters too.

If you encounter a mismatch:

- STOP and think deeply about why the plan can't be followed
- Present the issue clearly:

  ```
  Issue in Phase [N]:
  Expected: [what the plan says]
  Found: [actual situation]
  Why this matters: [explanation]

  How should I proceed?
  ```

## Verification Approach

After implementing a phase:

### Automated Verification:

- Run all commands listed under "Automated Verification" in the success criteria
- Ensure all tests pass and builds succeed
- Fix any issues before proceeding

### Manual Verification:

- Test features listed under "Manual Verification"
- Confirm functionality works as expected
- Test edge cases mentioned in the plan

### Progress Updates:

- Update your TodoWrite progress tracker
- Check off completed items in the plan file using Edit tool
- **REMINDER**: Only change `- [ ]` to `- [x]` in the plan - NO other modifications

Don't let verification interrupt your flow - batch it at natural stopping points.

## If You Get Stuck

When something isn't working as expected:

- First, make sure you've read and understood all the relevant code
- Consider if the codebase has evolved since the plan was written
- Present the mismatch clearly and ask for guidance

Use sub-tasks sparingly - mainly for targeted debugging or exploring unfamiliar territory.

## Resuming Work

If the plan has existing checkmarks:

- Trust that completed work is done
- Pick up from the first unchecked item
- Verify previous work only if something seems off

## Plan Integrity

The plan document is a historical contract that must remain unchanged except for checkbox updates:

- **Preserve all text exactly as written**
- **Only update checkbox states**: `- [ ]` → `- [x]`
- **No summaries or notes** in the plan document
- **No implementation details** added to the plan
- **No modifications to success criteria** or task descriptions

Use your TodoWrite tracker for implementation notes and progress details.

Remember: You're implementing a solution, not just checking boxes. Keep the end goal in mind and maintain forward momentum while preserving the plan's integrity.

---
description: "Create Architecture Decision Record (ADR) documentation for architectural decisions"
argument-hint: <decision_context_or_subject>
allowed-tools: Read(./**), Write(./.strategic-claude-basic/decisions/**), Bash(git:*, date:*, grep:*), Glob
model: claude-opus-4-5
---

You are tasked with creating focused, well-structured Architecture Decision Record (ADR) documentation for architectural decisions. You should be efficient, focused, and avoid extensive research - document the decision context you know.

**Decision Context/Subject provided:** $1

## Initial Response

When this command is invoked:

1. **Check if decision context/subject was provided**:

   - If context/subject was provided as parameter, proceed with analysis
   - If no context provided, respond with:

   ```
   I'll help you create an Architecture Decision Record (ADR) for an architectural decision.

   Please provide:
   1. A description of the architectural decision context, or
   2. A plan/research file path to extract decisions from

   Examples:
   - `/create_adr database technology selection for user service`
   - `/create_adr @.strategic-claude-basic/plan/PLAN_0001_07-09-2025_sat_user-auth.md`

   I'll document the architectural decision without extensive research.
   ```

   Then wait for the user's input.

## Process Steps

### Step 1: Input Analysis

1. **Determine input type**:

   **If it's a file path (starts with @. or contains .md)**:

   - Read the specified file completely
   - Look for sections about architectural decisions, technology choices, or design approaches
   - Prepare to create multiple ADRs if needed

   **If it's a decision context**:

   - Use the provided context as the decision subject
   - Look at conversation context for additional details about alternatives, rationale
   - Prepare to create a single ADR

2. **Present scope**:

   **For file input**:

   ```
   Reading file: [filename]

   Found [N] architectural decisions to document:
   - [Brief decision 1]
   - [Brief decision 2]
   - [Brief decision 3]

   I'll create individual ADR documents for each decision found.
   Proceeding with ADR creation...
   ```

   **For context input**:

   ```
   Creating ADR documentation for: [decision context]

   I'll document this architectural decision based on the current context and known details.
   Proceeding with ADR creation...
   ```

### Step 2: Decision Extraction and Analysis

**CRITICAL**: Do NOT spawn research agents or extensive analysis. Only use information already available.

1. **For plan/research files**:

   - Extract architectural decisions from "Technical Decisions" sections
   - Extract decisions from "Technology Stack" sections
   - Extract decisions from "Design Patterns" sections
   - Note any alternatives or rationale already mentioned

2. **For single decision contexts**:

   - Use conversation context to understand the decision
   - Look for any alternatives mentioned or discussed
   - Check for any rationale already provided

3. **Gather basic context**:

   - Affected components mentioned in conversation
   - Technology choices or patterns being decided
   - Any constraints or requirements mentioned

### Step 3: ADR Documentation

1. **Gather metadata for ADR documents**:

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

2. **For each ADR to create**:

   - Generate filename using pattern: `ADR_NNNN_DD-MM-YYYY_day_brief-subject.md`
   - Use template: `@.strategic-claude-basic/templates/commands/adr.template.md`
   - Replace ALL bracketed placeholders with actual information
   - Follow naming convention from: `@.strategic-claude-basic/decisions/CLAUDE.md`

3. **Decision handling**:

   **If alternatives and rationale are known from context**:

   - Document them in the Alternatives Considered section
   - Include rationale in the Decision section
   - Mark status as "proposed" (never create as "accepted")

   **If decision context is minimal**:

   - Document what's known about the decision
   - Use placeholder sections where information is missing
   - Mark areas that need further investigation

4. **Write documents**:

   - Write document to: `.strategic-claude-basic/decisions/[filename]`
   - Update the `@.strategic-claude-basic/decisions/CLAUDE.md` file with new document entries

### Step 4: Present Results

1. **For single ADR**:

   ```
   Created ADR document:
   `.strategic-claude-basic/decisions/ADR_NNNN_DD-MM-YYYY_day_brief-subject.md`

   Decision: [Brief description]
   Status: proposed
   Context: [Known/Minimal]

   The architectural decision is documented and ready for review.
   ```

2. **For multiple ADRs from file**:

   ```
   Created [N] ADR documents from file:

   - ADR_NNNN_DD-MM-YYYY_day_decision1.md - [Brief description]
   - ADR_NNNN_DD-MM-YYYY_day_decision2.md - [Brief description]
   - ADR_NNNN_DD-MM-YYYY_day_decision3.md - [Brief description]

   Decisions documented: [N] technology, [N] design, [N] infrastructure
   Status: All created as "proposed"

   All architectural decisions are documented and ready for review.
   ```

## Important Guidelines

1. **Be Efficient**:

   - No extensive research or agent spawning
   - Use only information from context and provided files
   - Document what you know, mark unknowns appropriately

2. **Be Focused**:

   - Create one ADR per distinct architectural decision
   - Keep decision descriptions clear and concise
   - Include alternatives only if already mentioned

3. **Be Consistent**:

   - Follow the ADR template exactly
   - Use proper naming conventions
   - Update CLAUDE.md tracking
   - Always start ADRs as "proposed"

4. **Be Practical**:

   - Include context based on what's described
   - Reference affected components when mentioned
   - Provide implementation approach when obvious

## Common Scenarios

### Scenario 1: Technology Decision from Conversation

When user describes a technology choice:

- Extract the technology options being considered
- Look for reasons or constraints mentioned
- Document any trade-offs already discussed
- Create single ADR document

### Scenario 2: Multiple Decisions from Plan

When user provides a plan file:

- Read plan completely
- Extract all distinct architectural decisions mentioned
- Create separate ADR for each decision
- Batch create all ADR documents

### Scenario 3: Decision Without Known Alternatives

When decision context is provided but alternatives aren't known:

- Document the decision context clearly
- Mark alternatives section as "pending investigation"
- Include what constraints or requirements were mentioned
- Set status as "proposed"

## Success Criteria

A good ADR document should:

- **Be actionable**: Clear decision statement and implementation approach
- **Be specific**: Reference exact technologies, patterns, or components
- **Be appropriate**: Correct status (always "proposed" initially)
- **Be complete**: All known information captured from context
- **Be ready**: Prepared for review and decision-making process

The goal is rapid documentation of architectural decisions, not comprehensive analysis. Capture what you know and create properly structured ADRs.

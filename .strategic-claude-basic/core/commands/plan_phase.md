---
description: "Create implementation plans for specific roadmap phases or milestones"
argument-hint: <phase_or_milestone_number> [--with-codex] [optional_focus_area]
allowed-tools: Read(./**), Write(.strategic-claude-basic/plan/**), Task, Bash(mkdir:*, date:*, git:*), Glob
model: claude-opus-4-5
---

You are tasked with creating focused implementation plans for specific phases or milestones from the product roadmap. You should validate product documentation exists, extract phase/milestone-specific requirements, and create detailed implementation plans aligned with the phase objectives and deliverables.

**Phase/Milestone provided:** $1

## Flag Parsing

- Check command arguments for `--with-codex`; if present, enable `CODEX_PHASE_PLAN_MODE=true`
- Strip the flag from phase/milestone parameters before proceeding with validation
- When `CODEX_PHASE_PLAN_MODE=true`, verify the `mcp__codex__codex` tool is available

  - If unavailable, inform the user:

    ```
    ❌ Codex mentorship unavailable — continuing with standard phase planning flow. Configure the Codex MCP server and rerun with `--with-codex` for guided validation.
    ```

  - After notifying the user, proceed in standard mode (treat `CODEX_PHASE_PLAN_MODE=false`)

- When the tool is available, keep Codex mentorship enabled for the remaining steps

## Initial Response

When this command is invoked:

1. **Check if phase/milestone number was provided**:

   - If a phase/milestone number was provided as parameter (e.g., "2.3", "1", "Phase 2.1", "3"), proceed with validation
   - If no phase/milestone number provided, respond with:

```
I'll help you create implementation plans for specific phases or milestones from your product roadmap.

Please provide:
1. The phase or milestone number you want to plan (e.g., "1", "1.2", "2.3", "3")
2. Optional focus area or specific aspect to emphasize

Examples:
- `/plan_phase 2.3` - Creates plan for milestone 2.3 only
- `/plan_phase 3` - Creates plans for all milestones in phase 3 (3.1, 3.2, 3.3)
- `/plan_phase 1.1 infrastructure` - Creates plan for milestone 1.1 with infrastructure focus

I'll validate your product documentation, extract phase/milestone details, and create targeted implementation plans.
```

Then wait for the user's input.

## Process Steps

### Step 1: Product Documentation Validation

1. **Check for required product documentation files**:

   - Look for existence of these files in `.strategic-claude-basic/product/`:
     - `PRD.md` (Product Requirements Document) - **REQUIRED**
     - `ARCHITECTURE.md` (System Architecture) - **REQUIRED**
     - `ROADMAP.md` (Product Roadmap) - **REQUIRED**
     - `REFERENCES.md` (Local Repository References) - **OPTIONAL**

2. **If any required files are missing**:

   ```
   Product documentation validation failed. Missing required files:
   - [ ] PRD.md - Product Requirements Document
   - [ ] ARCHITECTURE.md - System Architecture
   - [ ] ROADMAP.md - Product Roadmap

   To use phase-based planning, you need complete product documentation.

   Please first create your product documentation using:
   `/create_product_roadmap [research_file1].md [research_file2].md`

   This will generate all required product documentation files based on your research.
   ```

   Stop here and wait for user to create missing documentation.

3. **If all required files exist**, proceed to read them:

   - **IMPORTANT**: Use the Read tool WITHOUT limit/offset parameters to read entire files
   - Read PRD.md, ARCHITECTURE.md, and ROADMAP.md completely
   - Read REFERENCES.md if it exists
   - This provides full product context before phase planning

### Step 1.5: Architecture Decision Records (ADR) Review

1. **Discover and read all relevant ADRs**:

   - Use Glob to find all ADR files: `.strategic-claude-basic/decisions/ADR_*.md`
   - Read all ADRs with status: accepted, proposed (skip rejected, superseded)
   - Extract key decisions, rationale, and consequences that may impact phase planning
   - Note any decisions that directly affect the phase deliverables or technical approach

2. **If ADRs are found**, analyze their impact:

   ```
   Found [N] Architecture Decision Records:
   - ADR-NNNN: [Title] - [Status] - [Brief impact on planning]
   - ADR-NNNN: [Title] - [Status] - [Brief impact on planning]

   These decisions will be considered during phase planning to ensure alignment.
   ```

3. **If no ADRs found**, note this for future reference:

   ```
   No Architecture Decision Records found in .strategic-claude-basic/decisions/
   Phase planning will proceed without architectural constraints from ADRs.
   ```

### Step 2: Phase/Milestone Extraction and Validation

1. **Parse the provided phase/milestone number**:

   - Support multiple formats:
     - Simple numbers: "1", "2", "3" (full phase)
     - Decimal notation: "1.1", "2.3", "1.2" (specific milestone)
     - With Phase prefix: "Phase 1", "Phase 2.1"
     - With Milestone prefix: "Milestone 1.2"
   - Extract major phase number (e.g., "2" from "2.3")
   - Extract milestone number if provided (e.g., "3" from "2.3")

2. **Search for phase/milestone in ROADMAP.md**:

   - Look for phase headers matching patterns:
     - `## Phase [X]:` for major phases
     - `### Milestone [X.Y]:` or `### [Milestone X.Y]:` for specific milestones
     - Handle variations in formatting and naming

3. **If phase/milestone not found**:

   - Extract available phases and milestones from ROADMAP.md
   - Present clear options to user:

   ```
   Phase/Milestone [provided_number] not found in ROADMAP.md.

   Available phases and milestones in your roadmap:
   - Phase 1: [Phase Name] (Milestones: 1.1, 1.2, 1.3)
   - Phase 2: [Phase Name] (Milestones: 2.1, 2.2, 2.3)
   - Phase 3: [Phase Name] (Milestones: 3.1, 3.2)

   Please provide a valid phase or milestone number from the list above.
   ```

   Wait for user correction.

4. **Determine planning scope**:

   **For specific milestone (e.g., "2.3")**:

   - Extract single milestone content
   - Plan for that milestone only

   **For full phase (e.g., "2")**:

   - Extract all milestones in that phase
   - Plan to create separate plan files for each milestone

5. **Present planning scope**:

   **For single milestone**:

   ```
   Found Milestone [X.Y]: [Milestone Name]

   **Objectives**: [Milestone objectives]
   **Key Deliverables**:
   - [Deliverable 1]
   - [Deliverable 2]

   **Timeline**: [Target dates if specified]

   I'll create a focused implementation plan for this milestone.
   ```

   **For full phase**:

   ```
   Found Phase [X]: [Phase Name]

   **Milestones to plan**:
   - Milestone [X.1]: [Name] - [Brief description]
   - Milestone [X.2]: [Name] - [Brief description]
   - Milestone [X.3]: [Name] - [Brief description]

   I'll create separate implementation plans for each milestone in this phase.
   ```

### Step 2.5: Codex Mentorship Alignment (when CODEX_PHASE_PLAN_MODE=true)

Before committing to phase/milestone planning approach, collaborate with the Codex MCP mentor:

- Prepare a concise status brief for Codex including:
  - Phase/milestone objectives, deliverables, and timeline from ROADMAP.md
  - Current product understanding from PRD.md and ARCHITECTURE.md insights
  - Milestone scope and planning approach determined in Step 2
  - Known constraints, dependencies, or risks from research documents
- Use `mcp__codex__codex` (or a Task agent that delegates to Codex) to evaluate the planning situation. Request that Codex:
  - Challenge assumptions and highlight blind spots or missing dependencies for the phase/milestone
  - Recommend optimal planning approach that aligns with existing architecture and product direction
  - Call out required sequencing changes, prerequisite work, or cross-team coordination needed
  - Point to best-practice implementations for frameworks/libraries that may appear in milestone deliverables
- Example mentor prompt:

  ```
  mcp__codex__codex(
    prompt: "You're my phase planning mentor. Given this context: [phase/milestone summary, objectives, deliverables, timeline, candidate planning approach],
    help me decide on the optimal planning strategy for this phase/milestone. Identify risks, blockers, or missing investigations.
    Flag any architectural or dependency areas where careful consideration will be critical. Challenge assumptions before I create the implementation plans.",
    approval-policy: "never",
    sandbox: "workspace-write"
  )
  ```

- Capture Codex's guidance (especially blockers, missing research, architectural considerations) and resolve open items before moving to Step 3
- If Codex proposes additional investigation, return to Step 2 to gather the required evidence before proceeding

### Step 3: Research Discovery and Integration

1. **Search for phase/milestone-related research documents**:

   - Look in `.strategic-claude-basic/research/` for files related to the phase
   - Search patterns:
     - Exact match: "phase[X]", "phase-[X]", "milestone-[X]-[Y]"
     - Partial match: "[X].[Y]" in filename
     - Content search: phase/milestone numbers in document content

2. **Read all related research documents**:

   - **IMPORTANT**: Read completely, not partially
   - Extract research relevant to the specific phase/milestone
   - Note technical findings and implementation recommendations

3. **Create planning research todo list** using TodoWrite to track planning tasks

4. **Spawn parallel sub-tasks for comprehensive planning research**:

   **For each milestone being planned:**

   - Use the **codebase-locator** agent to find files related to milestone deliverables
   - Use the **codebase-analyzer** agent to understand current implementation state
   - Use the **codebase-pattern-finder** agent to find similar implementation patterns

   Each research task should:

   - Focus on specific milestone deliverables and success criteria
   - Find relevant code examples and implementation patterns
   - Identify technical challenges and solutions
   - Return specific file:line references
   - Understand integration points and dependencies
   - **For test planning**: Identify existing test patterns, frameworks, and coverage gaps
   - **For test planning**: Find test utilities, fixtures, and validation approaches used in codebase

5. **Wait for ALL sub-tasks to complete** before proceeding with planning

### Step 4: Implementation and Test Plan Generation

1. **For each milestone to be planned**:

   - Gather metadata for plan document:

     ```bash
     # Get current date/time with timezone
     date --iso-8601=seconds

     # Get current git commit hash
     git rev-parse HEAD

     # Get current branch name
     git branch --show-current

     # Get next document number and create filename
     date '+%d-%m-%Y-%a' | tr '[:upper:]' '[:lower:]'
     ```

   - Generate subject name from milestone:
     - Milestone 1.1 → "phase1-1-project-infrastructure"
     - Milestone 2.3 → "phase2-3-motion-blur-integration"
     - Full Phase 3 → "phase3-1-interactive-controls", "phase3-2-visual-themes", "phase3-3-mobile-optimization"

2. **Create implementation plan document**:

   - Use template: `@.strategic-claude-basic/templates/commands/plan.template.md`
   - Replace ALL bracketed placeholders with actual milestone details
   - Follow naming convention: `PLAN_NNNN_DD-MM-YYYY_day_phase[X]-[Y]-[subject].md`
   - Include relevant ADR references in frontmatter and content
   - Focus on building phases, architecture, and implementation approach
   - Ensure approach aligns with accepted architectural decisions

3. **Create test plan document for the same milestone**:

   - Use template: `@.strategic-claude-basic/templates/commands/test_plan.template.md`
   - Replace ALL bracketed placeholders with test-specific milestone details
   - Follow naming convention: `TEST_NNNN_DD-MM-YYYY_day_phase[X]-[Y]-[subject].md` (same number as implementation plan)
   - Cross-reference the implementation plan in References section
   - Include ADR compliance testing where architectural decisions require validation
   - Focus on test coverage, validation strategy, and quality assurance for milestone deliverables

4. **Structure both plans around milestone deliverables**:

   For each major deliverable:

   - Current state analysis (what exists today)
   - Implementation approach (how to build it)
   - Technical challenges (what obstacles exist)
   - ADR compliance (how approach aligns with architectural decisions)
   - Success validation (automated and manual criteria)
   - Integration points (how it connects to other components)

5. **Write both plan documents to**: `.strategic-claude-basic/plan/` using the naming conventions

   - Implementation plan: `PLAN_NNNN_date_phase[X]-[Y]-subject.md`
   - Test plan: `TEST_NNNN_date_phase[X]-[Y]-subject.md`

6. **Update the planning registry**: Add both entries to `.strategic-claude-basic/plan/CLAUDE.md`

### Step 4.5: Codex Plan Validation (when CODEX_PHASE_PLAN_MODE=true)

Before presenting the plans to the user, run a Codex validation pass:

- Prepare a structured summary for Codex containing:
  - Phase/milestone overview, key deliverables, and success criteria (automated + manual)
  - Notable risks, dependencies, and integration points documented in the plans
  - Any code snippets, architectural decisions, or library usages introduced
  - References to supporting research/product docs and critical ADR numbers
- Invoke `mcp__codex__codex` (or a Task agent that delegates to Codex) with instructions to:
  - Review the phase/milestone plans for gaps, blockers, missing prerequisites, or misordered work
  - Verify the plans align with the current product roadmap and milestone objectives
  - Confirm architectural approach follows best practices and roadmap direction
  - Highlight missing success criteria, testing obligations, or delivery considerations
  - Ensure risks, mitigation steps, and scope boundaries are explicitly covered
- Example validation prompt:

  ```
  mcp__codex__codex(
    prompt: "Review these phase/milestone implementation and test plans. Identify blockers, missing steps, or misalignments with the product roadmap and milestone objectives.
    Check that each deliverable is feasible, dependencies are satisfied, and architectural approach follows best practices.
    Call out any unclear success criteria or testing gaps. Suggest refinements that make the plans execution-ready for this specific phase/milestone.",
    approval-policy: "never",
    sandbox: "workspace-write"
  )
  ```

- Integrate Codex feedback into the plans before finalizing. If Codex raises concerns that require product/business decisions, pause and consult the user instead of forcing a decision.
- Capture the key Codex findings (especially blockers or architectural corrections) to relay in your final response.

### Step 5: Plan Presentation and Completion

1. **For single milestone planning**:

   ```
   I've created both implementation and test plans for Milestone [X.Y]: [Milestone Name]

   **Implementation Plan**: `.strategic-claude-basic/plan/PLAN_[NNNN]_[date]_phase[X]-[Y]-[subject].md`
   - Focuses on building the milestone deliverables
   - Implementation phases and technical approach
   - Integration points and dependencies

   **Test Plan**: `.strategic-claude-basic/plan/TEST_[NNNN]_[date]_phase[X]-[Y]-[subject].md`
   - Focuses on validating the milestone deliverables
   - Test coverage strategy and test types
   - Quality assurance and verification procedures

   **Key Implementation Areas**:
   - [Technical area 1 with specific approach]
   - [Technical area 2 with integration points]
   - [Technical area 3 with success criteria]

   **Test Coverage Areas**:
   - [Test area 1 with coverage strategy]
   - [Test area 2 with validation approach]
   - [Test area 3 with quality metrics]

   Both plans are ready for execution and include specific file paths, code examples, and comprehensive verification procedures.
   ```

   - When `CODEX_PHASE_PLAN_MODE=true`, include a dedicated section in your response summarizing Codex's mentorship and validation feedback. Explicitly mention any adjustments made or blockers that remain so the user understands Codex's contribution to the phase/milestone planning.

2. **For full phase planning**:

   ```
   I've created both implementation and test plans for all milestones in Phase [X]: [Phase Name]

   **Plan Documents Created**:

   **Milestone [X.1]: [Milestone X.1 Name]**
   - Implementation: `.strategic-claude-basic/plan/PLAN_NNNN_..._phase[X]-1-[name].md`
   - Test: `.strategic-claude-basic/plan/TEST_NNNN_..._phase[X]-1-[name].md`

   **Milestone [X.2]: [Milestone X.2 Name]**
   - Implementation: `.strategic-claude-basic/plan/PLAN_NNNN+1_..._phase[X]-2-[name].md`
   - Test: `.strategic-claude-basic/plan/TEST_NNNN+1_..._phase[X]-2-[name].md`

   **Milestone [X.3]: [Milestone X.3 Name]**
   - Implementation: `.strategic-claude-basic/plan/PLAN_NNNN+2_..._phase[X]-3-[name].md`
   - Test: `.strategic-claude-basic/plan/TEST_NNNN+2_..._phase[X]-3-[name].md`

   **Phase Overview**:
   - Total Documents: [6 documents - 3 implementation + 3 test plans]
   - Total Deliverables: [Number across all milestones]
   - Implementation Timeline: [Phase duration]
   - Test Coverage: [Comprehensive testing strategy across milestones]

   Each plan pair is milestone-focused with implementation providing technical approach and test plan providing comprehensive validation strategy.
   ```

   - When `CODEX_PHASE_PLAN_MODE=true`, include a dedicated section in your response summarizing Codex's mentorship and validation feedback across all milestone plans. Explicitly mention any adjustments made or blockers that remain so the user understands Codex's contribution to the phase planning.

## Important Guidelines

1. **Be Milestone-Focused**:

- All planning should directly support the specified milestone objectives
- Prioritize deliverables that impact milestone success criteria
- Connect technical solutions to business outcomes defined in the milestone

2. **Be ADR-Compliant**:

- All implementation approaches must align with accepted architectural decisions
- Flag any conflicts between milestone requirements and existing ADRs
- Reference relevant ADR numbers (ADR-NNNN) when decisions influence approach
- Note when new ADRs may be needed for milestone-specific decisions

3. **Be Implementation-Ready**:

- Provide actionable implementation steps for milestone deliverables
- Include specific code examples and patterns from existing codebase
- Identify reusable components and utilities
- Define clear success criteria (automated and manual)

4. **Be Research-Informed**:

- Read ALL related research documents before planning
- Include specific file paths and line numbers for implementation references
- Cross-reference technical solutions with research findings
- Address technical challenges identified in research

5. **Be Scope-Appropriate**:

- Single milestone plans focus exclusively on that milestone
- Full phase plans create separate, focused plans per milestone
- Include clear "what we're NOT doing" sections to prevent scope creep
- Balance comprehensiveness with milestone-specific relevance

## Plan Quality Standards

### For Single Milestone Plans:

**Implementation Plan (PLAN\_):**

- [ ] Milestone objectives clearly addressed
- [ ] All deliverables have implementation approaches
- [ ] Success criteria include automated and manual verification
- [ ] Dependencies and integration points identified
- [ ] Technical challenges from research addressed
- [ ] Specific file:line references provided for implementation

**Test Plan (TEST\_):**

- [ ] Test coverage aligns with milestone deliverables
- [ ] Test strategy addresses implementation complexity
- [ ] Test data and fixture requirements defined
- [ ] Cross-references implementation plan in References section
- [ ] Test automation and CI integration specified
- [ ] Quality metrics and validation criteria established

### For Full Phase Plans:

- [ ] Separate implementation and test plan pair created for each milestone in the phase
- [ ] Each plan maintains milestone-specific focus
- [ ] Cross-milestone dependencies identified in implementation plans
- [ ] Phase-level integration testing considerations included in test plans
- [ ] Sequential numbering follows CLAUDE.md conventions (shared numbering for pairs)
- [ ] All implementation plans meet implementation plan standards
- [ ] All test plans meet test plan standards
- [ ] Plan pairs cross-reference each other appropriately

## Milestone Planning Focus Areas

### For Infrastructure Milestones (typically X.1):

- Development environment and tooling setup
- Build system configuration and optimization
- Dependency management and integration
- Basic project structure and organization

### For Core Feature Milestones (typically X.2):

- Primary feature implementation and logic
- Data models and state management
- API design and integration points
- Core user experience components

### For Integration/Polish Milestones (typically X.3):

- System integration and testing
- Performance optimization and tuning
- User interface polish and enhancement
- Documentation and deployment preparation

### For Advanced Feature Milestones (typically X.4+):

- Advanced functionality and edge cases
- Scalability and performance enhancements
- Complex integrations and external services
- Specialized tooling and utilities

## Example Phase/Milestone Patterns

### Foundation Phase (Phase 1):

- 1.1: Project Infrastructure Setup
- 1.2: Basic System Implementation
- 1.3: Core Algorithm/Logic Implementation

### Development Phase (Phase 2):

- 2.1: Advanced System Implementation
- 2.2: Feature Enhancement and Optimization
- 2.3: Integration and Effects Implementation

### Enhancement Phase (Phase 3):

- 3.1: Interactive Features and Controls
- 3.2: Visual Enhancements and Themes
- 3.3: Platform Optimization (Mobile/Desktop)

### Production Phase (Phase 4):

- 4.1: Testing and Quality Assurance
- 4.2: Performance Optimization and Tuning
- 4.3: Documentation and Deployment

---

**Planning Focus**: Milestone-specific implementation plans with actionable steps
**Output**: Focused implementation plans aligned with milestone objectives and deliverables
**Success Measure**: Plans directly enable milestone implementation and success criteria validation

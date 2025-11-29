---
description: "Analyze plans for potential blocking issues by examining codebase, dependencies, and related documents"
argument-hint: <plan_file(s)_or_NNNN> [--with-codex]
allowed-tools: Read(./**), Task, Bash(git:*, find:*, grep:*), Glob, Grep, mcp__codex__codex
model: claude-opus-4-5
---

You are tasked with analyzing implementation plans for potential blocking issues by examining the codebase, technical dependencies, related summaries, and research documents.

**Plan input provided:** $1

## Input Format Support

This command supports multiple input formats:

- **Single plan**: `/check_plan_for_blockers PLAN_0019_16-09-2025-tue_reed-solomon-aligned-strategy.md`
- **Multiple plans**: `/check_plan_for_blockers PLAN_0019_16-09-2025-tue_reed-solomon-aligned-strategy.md TEST_0019_16-09-2025-tue_rs-aligned-strategy-testing.md`
- **NNNN number**: `/check_plan_for_blockers 0019` (finds all plans with that number)
- **Multiple NNNN**: `/check_plan_for_blockers 0019 0020 0021`
- **With Codex analysis**: `/check_plan_for_blockers 0019 --with-codex` (adds alternative Codex perspective)

### Flag Options

- **--with-codex**: Enables dual-source analysis using the Codex MCP server for an alternative perspective on blockers and dependencies

## Initial Response

When this command is invoked:

1. **Check if plan input was provided**:

   - If plan input provided, proceed with analysis
   - If no input provided, respond with:

   ```
   I'll analyze implementation plans for potential blocking issues by examining the codebase, dependencies, and related work.

   Please provide plan file(s) or NNNN number(s) to analyze:
   Examples:
   - Single plan: `/check_plan_for_blockers PLAN_0019_16-09-2025-tue_reed-solomon-aligned-strategy.md`
   - Multiple plans: `/check_plan_for_blockers PLAN_0019... TEST_0019...`
   - NNNN number: `/check_plan_for_blockers 0019`
   - Multiple NNNN: `/check_plan_for_blockers 0019 0020 0021`
   - With Codex analysis: `/check_plan_for_blockers 0019 --with-codex`

   I'll use codebase agents and optionally Codex analysis to identify blockers and propose plan updates.
   ```

   Then wait for the user's input.

## Process Steps

### Step 1: Plan Discovery and Validation

1. **Parse input arguments**:

   - If argument contains `.md`, treat as plan file path(s)
   - If argument is 4-digit number (NNNN), search for matching plans
   - Support space-separated multiple inputs
   - Check for `--with-codex` flag and set codex analysis mode

2. **Find and validate plan files**:

   ```bash
   # For NNNN inputs, find matching plans
   find .strategic-claude-basic/plan/ -name "*_NNNN_*" -type f

   # Validate plan files exist
   test -f ".strategic-claude-basic/plan/[plan-file]"
   ```

3. **Present plan discovery results**:

   ```
   Plan Discovery Results:
   Found [N] plan(s) for analysis:
   - PLAN_NNNN_date_subject.md ([implementation plan])
   - TEST_NNNN_date_subject.md ([test plan])

   Proceeding with blocker analysis...
   ```

### Step 2: Document Context Analysis

1. **Read all discovered plans completely**:

   - Extract implementation requirements and phases
   - Identify technical dependencies mentioned
   - Note file paths and components to be modified
   - Extract success criteria and prerequisites

2. **Find related documents**:

   **Summary Documents**:

   - Search for matching SUMMARY documents with same NNNN
   - Read summaries to understand current status and known issues
   - Extract incomplete work and technical debt

   **Research Documents**:

   - Search for matching RESEARCH documents
   - Read research to understand design decisions and constraints

3. **Present document context**:

   ```
   Document Context Analysis:
   Plans to analyze:
   - [Plan 1]: [brief description of scope]
   - [Plan 2]: [brief description of scope]

   Related documents found:
   - Summaries: [N] found ([completion status])
   - Research: [N] found ([topics covered])

   Proceeding with codebase dependency analysis...
   ```

### Step 3: Codebase Dependency Analysis

1. **Use codebase agents for deep analysis**:

   **For each plan, analyze dependencies using agents**:

   ```
   # Use codebase-locator to find relevant files
   Task: codebase-locator
   Prompt: "Find files related to [plan scope] including [key components from plan]"

   # Use codebase-analyzer for implementation details
   Task: codebase-analyzer
   Prompt: "Analyze implementation status and dependencies for [specific components]. Focus on interface compatibility, missing implementations, and integration points."

   # Use codebase-pattern-finder for similar implementations
   Task: codebase-pattern-finder
   Prompt: "Find existing patterns for [plan approach] that could guide or block implementation"
   ```

2. **Git history analysis**:

   ```bash
   # Check recent commits that might affect dependencies
   git log --oneline -20

   # Look for ongoing work that might conflict
   git status --porcelain

   # Check for unstaged changes in relevant areas
   git diff --name-only
   ```

### Step 4: Codex Alternative Analysis (if --with-codex flag present)

1. **Run Codex analysis for alternative perspective**:

   ```
   # Use Codex MCP server for deep architectural analysis
   mcp__codex__codex
   Prompt: "Analyze the following implementation plan(s) for potential blocking issues:

   [Plan content summary]

   Focus on:
   - Missing prerequisites and dependencies
   - Interface incompatibilities and API conflicts
   - Architectural concerns and design conflicts
   - Hidden dependencies not obvious from surface analysis
   - Performance bottlenecks and resource constraints
   - Integration challenges with existing systems
   - Testing infrastructure requirements
   - Potential regression risks

   Provide specific technical concerns with file paths and concrete examples where possible."
   ```

2. **Codex analysis advantages**:

   - **Architectural perspective**: Broader view of system interactions
   - **Pattern recognition**: Identifies anti-patterns and design conflicts
   - **Deep dependency analysis**: Finds non-obvious interdependencies
   - **Alternative viewpoint**: May catch issues standard analysis misses

3. **Present Codex findings**:

   ```
   Codex Alternative Analysis:
   🔍 Additional blockers identified:
   - [Codex-specific finding 1]
   - [Codex-specific finding 2]

   🔄 Conflicting assessments:
   - Standard analysis: [finding]
   - Codex analysis: [alternative view]

   ✅ Consensus issues (found by both):
   - [High-confidence blocker 1]
   - [High-confidence blocker 2]
   ```

### Step 5: Technical Blocker Identification

1. **Categorize potential blockers**:

   **Critical Blockers** 🔴:

   - Missing required dependencies
   - Incompatible interfaces or APIs
   - Test failures in prerequisite components
   - Conflicting ongoing work

   **Non-Critical Issues** 🟡:

   - Performance concerns
   - Technical debt
   - Minor API inconsistencies
   - Documentation gaps

   **Prerequisites** ⚠️:

   - Required setup or configuration
   - Dependency updates needed
   - Environmental requirements

2. **Test current codebase state**:

   ```bash
   # Run basic build check
   go build ./...

   # Check for linting issues in relevant areas
   golangci-lint run [relevant-paths]

   # Quick test check for critical components
   go test -short [relevant-packages]
   ```

### Step 6: Agent-Enhanced Analysis

1. **Use specialized agents for comprehensive analysis**:

   **For each identified dependency**:

   - **codebase-analyzer**: Analyze implementation completeness
   - **codebase-locator**: Find all related files and interfaces
   - **general-purpose**: Research complex integration points

2. **Cross-reference with known issues**:

   - Check summaries for unresolved critical issues
   - Look for TODO/FIXME comments in related code
   - Identify performance bottlenecks or limitations

### Step 7: Comprehensive Blocker Report with Plan Updates

1. **Generate consolidated blocker assessment**:

   ```
   ## Consolidated Plan Blocker Analysis: [Plan Names]

   ### 📊 Analysis Sources
   - ✅ Standard codebase analysis completed
   - [✅/➖] Codex alternative analysis [completed/not requested]

   ### ✅ Prerequisites Status
   - [Dependency 1]: ✅ Available / ❌ Missing / ⚠️ Partial
   - [Dependency 2]: ✅ Available / ❌ Missing / ⚠️ Partial

   ### 🔴 Critical Blockers (Consolidated)
   **Issues that prevent implementation:**

   **High Confidence (Multiple Sources):**
   1. **[Consensus Blocker]** - [Description]
      - **Sources**: Standard + Codex analysis
      - **Impact**: [How it blocks the plan]
      - **Location**: [File/component affected]
      - **Resolution**: [Required action]
      - **Estimated Effort**: [Time/complexity]

   **Standard Analysis Only:**
   2. **[Standard Blocker]** - [Description]
      - **Source**: Codebase agents
      - **Confidence**: Medium

   **Codex Analysis Only:**
   3. **[Codex Blocker]** - [Description]
      - **Source**: Codex architectural analysis
      - **Confidence**: Medium

   ### 🟡 Non-Critical Issues
   **Issues that may complicate but don't block implementation:**
   1. **[Issue Name]** - [Description]
      - **Impact**: [How it affects implementation]
      - **Workaround**: [Possible solutions]

   ### ⚠️ Prerequisites & Setup
   **Required before starting implementation:**
   1. **[Prerequisite]** - [Description and setup steps]

   ### 📋 Implementation Readiness
   - **Ready to proceed**: ✅ / ❌
   - **Critical blockers**: [N] found ([X] consensus, [Y] single-source)
   - **Dependencies available**: [N/Total]
   - **Risk level**: Low / Medium / High

   ### 🔧 Proposed Plan Updates

   **Current Plan Structure:**
   - Phase 1: [Current Phase 1 description]
   - Phase 2: [Current Phase 2 description] ← **BLOCKERS DETECTED**
   - Phase 3: [Current Phase 3 description]

   **Recommended Plan Updates:**
   - Phase 1: [Unchanged]
   - **Phase 1.5: Blocker Resolution** ← **NEW PHASE**
     - [ ] Resolve [Blocker 1]: [Specific resolution steps]
     - [ ] Fix [Dependency Issue]: [Specific fix approach]
     - [ ] Update [Interface]: [Compatibility fixes]
     - [ ] Test blocker resolutions
   - Phase 2: [Modified - now can proceed safely]
   - Phase 3: [Unchanged]

   ### 🚀 Recommended Actions
   1. **Immediate**: Address critical blockers in Phase 1.5
   2. **During implementation**: Monitor resolved dependencies
   3. **Future**: [Items to address in subsequent work]

   ---
   **📝 Plan Update Proposal Available**
   Would you like me to update the plan to fix blockers? (Y/n)
   ```

2. **Agent-sourced evidence**:

   - Include specific findings from codebase agents
   - Reference exact file locations and line numbers
   - Provide code examples of potential conflicts

### Step 8: Interactive Plan Update Flow

1. **Present plan update proposal**:

   After completing the blocker analysis, always present a plan update proposal that includes:

   - Identification of which phase has blockers
   - Creation of intermediate phases (e.g., Phase 1.5, Phase 2.5) to resolve blockers
   - Specific actionable tasks for each blocker resolution
   - Updated phase dependencies and prerequisites

2. **User interaction handling**:

   ```
   **📝 Plan Update Proposal Available**
   Would you like me to update the plan to fix blockers? (Y/n)
   ```

   **If user responds "Y" or "yes"**:

   - Automatically generate updated plan files
   - Create new phases with blocker resolution tasks
   - Update existing phases to reflect resolved dependencies
   - Generate implementation roadmap

   **If user responds "n" or "no"**:

   - Provide summary of manual steps they can take
   - Offer to export blocker report for reference

3. **Automatic plan update generation**:

   When user approves updates:

   - Read original plan file(s)
   - Insert new resolution phases at appropriate points
   - Update phase numbering and dependencies
   - Add specific tasks for each identified blocker
   - Preserve original plan structure and formatting
   - Create backup of original plan

4. **Phase insertion logic**:

   ```
   Original Plan:
   - Phase 1: Setup
   - Phase 2: Core Implementation ← BLOCKERS HERE
   - Phase 3: Testing

   Updated Plan:
   - Phase 1: Setup
   - Phase 1.5: Blocker Resolution
     - [ ] Fix dependency X
     - [ ] Resolve interface Y
     - [ ] Update configuration Z
   - Phase 2: Core Implementation (now safe to proceed)
   - Phase 3: Testing
   ```

5. **Generated tasks format**:

   For each blocker, create specific, actionable tasks:

   ```
   ### Phase 1.5: Blocker Resolution

   #### Fix [Blocker Name]
   - [ ] **Analyze**: [Specific analysis steps]
   - [ ] **Implement**: [Specific implementation steps]
   - [ ] **Test**: [Specific testing steps]
   - [ ] **Validate**: [Specific validation criteria]

   **Acceptance Criteria:**
   - [ ] [Specific measurable outcome 1]
   - [ ] [Specific measurable outcome 2]
   ```

## Advanced Analysis Features

### Multi-Plan Cross-Dependencies

When analyzing multiple plans:

1. **Check for plan interdependencies**:

   - Plans that modify the same components
   - Sequential dependencies between plans
   - Resource conflicts (same files, interfaces)

2. **Optimization opportunities**:
   - Shared implementations
   - Combined testing strategies
   - Coordinated development approach

### Performance Impact Analysis

For performance-sensitive plans:

1. **Use performance-focused agents**:

   - Analyze current performance baselines
   - Identify potential performance regressions
   - Evaluate resource requirements

2. **Benchmark existing similar code**:
   - Find performance-critical paths
   - Analyze memory and CPU usage patterns

### Integration Readiness

For plans involving multiple components:

1. **Interface compatibility analysis**:

   - Check API version compatibility
   - Analyze data structure alignment
   - Verify protocol consistency

2. **Testing infrastructure readiness**:
   - Check test coverage of dependencies
   - Verify testing tools availability
   - Assess CI/CD pipeline compatibility

## Error Handling

### Plan Not Found

```
❌ Plan file not found: [filename]

Available plans in .strategic-claude-basic/plan/:
[List of available plans with NNNN numbers]

Please check the filename or NNNN number and try again.
```

### Multiple NNNN Matches

```
⚠️ Multiple plans found for NNNN [number]:
- PLAN_NNNN_date_subject.md
- TEST_NNNN_date_subject.md
- [Additional matches]

Analyzing all found plans...
```

### Agent Analysis Failures

```
⚠️ Codebase agent analysis encountered issues:
- [Agent name]: [Error or limitation]
- Falling back to manual analysis for affected areas
```

## Success Criteria

A successful blocker analysis should:

- **Identify all critical blockers** that prevent implementation
- **Provide actionable resolution steps** for each blocker
- **Use agent insights** to validate technical assumptions
- **Leverage Codex analysis** when requested for alternative perspectives
- **Consolidate findings** from multiple analysis sources with confidence levels
- **Generate specific plan updates** with intermediate blocker resolution phases
- **Offer interactive plan modification** with user approval workflow
- **Provide implementation readiness assessment** with confidence level
- **Suggest optimization opportunities** for multi-plan scenarios
- **Reference specific code locations** and concrete evidence
- **Always propose plan updates** to resolve identified blockers

The analysis should enable informed decision-making about when and how to proceed with implementation work, and provide a clear path forward through automatic plan updates that address all identified blockers.

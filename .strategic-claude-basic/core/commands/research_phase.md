---
description: "Conduct focused research on a specific product roadmap phase"
argument-hint: <phase_number> [optional_focus_area]
allowed-tools: Read(./**), Write(./strategic-claude-basic/research/**), Task, Bash(mkdir:*), Glob
model: claude-opus-4-5
---

You are tasked with conducting focused research on a specific phase from the product roadmap. You should validate product documentation exists, extract phase-specific requirements, and conduct comprehensive research aligned with the phase objectives and deliverables.

**Phase number provided:** $1

## Initial Response

When this command is invoked:

1. **Check if phase number was provided**:

   - If a phase number was provided as parameter (e.g., "2.3", "1", "Phase 2.1"), proceed with validation
   - If no phase number provided, respond with:

```
I'll help you research a specific phase from your product roadmap.

Please provide:
1. The phase number you want to research (e.g., "1", "1.2", "2.3")
2. Optional focus area or specific aspect to emphasize

Example: `/research_phase 2.3` or `/research_phase 1.1 architecture`

I'll validate your product documentation, extract phase details, and conduct focused research on that phase.
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

   To use phase-based research, you need complete product documentation.

   Please first create your product documentation using:
   `/create_product_roadmap [research_file1].md [research_file2].md`

   This will generate all required product documentation files based on your research.
   ```

   Stop here and wait for user to create missing documentation.

3. **If all required files exist**, proceed to read them:

   - **IMPORTANT**: Use the Read tool WITHOUT limit/offset parameters to read entire files
   - Read PRD.md, ARCHITECTURE.md, and ROADMAP.md completely
   - Read REFERENCES.md if it exists
   - This provides full product context before phase research

### Step 1.5: Architecture Decision Records (ADR) Review

1. **Discover and read all relevant ADRs**:

   - Use Glob to find all ADR files: `.strategic-claude-basic/decisions/ADR_*.md`
   - Read all ADRs with status: accepted, proposed (skip rejected, superseded)
   - Extract key decisions, rationale, and consequences that may impact phase research
   - Note any decisions that directly affect the phase deliverables or approach

2. **If ADRs are found**, analyze their relevance to the phase:

   ```
   Found [N] Architecture Decision Records:
   - ADR-NNNN: [Title] - [Status] - [Impact on phase research]
   - ADR-NNNN: [Title] - [Status] - [Impact on phase research]

   These architectural decisions will guide the research approach for this phase.
   ```

3. **If no ADRs found**, note this for context:

   ```
   No Architecture Decision Records found in .strategic-claude-basic/decisions/
   Phase research will proceed without architectural decision constraints.
   ```

### Step 2: Phase Extraction and Validation

1. **Parse the provided phase number**:

   - Support multiple formats:
     - Simple numbers: "1", "2", "3"
     - Decimal notation: "1.1", "2.3", "1.2"
     - With Phase prefix: "Phase 1", "Phase 2.1"
     - With Milestone prefix: "Milestone 1.2"
   - Extract major phase number (e.g., "2" from "2.3")
   - Extract milestone number if provided (e.g., "3" from "2.3")

2. **Search for phase in ROADMAP.md**:

   - Look for phase headers matching patterns:
     - `## Phase [X]:` for major phases
     - `### [Milestone X.Y]:` or `### Milestone [X.Y]:` for specific milestones
     - Handle variations in formatting and naming

3. **If phase not found**:

   - Extract available phases and milestones from ROADMAP.md
   - Present clear options to user:

   ```
   Phase [provided_number] not found in ROADMAP.md.

   Available phases in your roadmap:
   - Phase 1: [Phase Name] (Milestones: 1.1, 1.2)
   - Phase 2: [Phase Name] (Milestones: 2.1, 2.2, 2.3)
   - Phase 3: [Phase Name] (Milestones: 3.1)

   Please provide a valid phase number from the list above.
   ```

   Wait for user correction.

4. **If phase found, extract phase content**:

   - Extract phase name and description
   - Extract objectives and key deliverables
   - Extract success criteria (automated and manual)
   - Extract dependencies and risks
   - Extract timeline and target dates

5. **Present extracted phase information**:

   ```
   Found Phase [X.Y]: [Phase Name]

   **Objectives**: [Phase objectives]
   **Key Deliverables**:
   - [Deliverable 1]
   - [Deliverable 2]
   - [Deliverable 3]

   **Success Criteria**: [Number] automated, [Number] manual verification steps
   **Dependencies**: [Dependencies if any]
   **Timeline**: [Target dates if specified]

   I'll now conduct focused research to support this phase implementation.
   ```

### Step 3: Phase-Focused Research Planning

1. **Analyze phase requirements and create research focus areas**:

   - **Technical Research**: Based on deliverables requiring implementation
   - **Architecture Research**: Based on system design and integration needs
   - **Dependency Research**: Based on external or internal dependencies
   - **Risk Research**: Based on identified risks and mitigation needs
   - **Performance Research**: Based on success criteria and metrics

2. **Create a phase research todo list** using TodoWrite to track all research tasks

3. **Spawn parallel sub-tasks for comprehensive phase research**:

   Before conducting research, use specialized agents to gather phase-specific information:

   **For technical implementation research:**

   - Use the **codebase-locator** agent to find files related to phase deliverables
   - Use the **codebase-analyzer** agent to understand current implementation of related features
   - Use the **codebase-pattern-finder** agent to find examples of similar implementations

   **For architectural and integration research:**

   - Use the **codebase-analyzer** agent to understand system architecture constraints
   - Use the Task Tool to find existing architectural documentation related to phase requirements
   - Use the **codebase-pattern-finder** agent to find integration patterns and examples

   Each research task should:

   - Focus on specific phase deliverables and success criteria
   - Find relevant code examples and implementation patterns
   - Identify technical challenges and solutions
   - Return specific file:line references
   - Understand integration points and dependencies related to the phase

4. **Wait for ALL sub-tasks to complete** before proceeding with analysis

### Step 4: Phase Research Synthesis and Documentation

1. **Synthesize research findings with phase context**:

   - **IMPORTANT**: Wait for ALL sub-agent tasks to complete before proceeding
   - Cross-reference research findings with phase objectives and deliverables
   - Identify how current codebase supports or constrains phase implementation
   - Map technical solutions to specific phase success criteria
   - Highlight gaps between phase requirements and current capabilities

2. **Structure findings around phase deliverables**:

   For each major deliverable:

   - Current state analysis (what exists today)
   - Implementation approach (how to build it)
   - Technical challenges (what obstacles exist)
   - ADR compliance considerations (how approach aligns with architectural decisions)
   - Success validation (how to verify completion)
   - Integration points (how it connects to other components)

3. **Gather metadata for research document**:

   Run these commands to gather all required metadata before writing the document:

   ```bash
   # Get current date/time with timezone
   date --iso-8601=seconds

   # Get current git commit hash
   git rev-parse HEAD

   # Get current branch name
   git branch --show-current

   # Get repository name (from remote origin)
   basename -s .git $(git config --get remote.origin.url)

   # Get next document number and create filename
   date '+%d-%m-%Y-%a' | tr '[:upper:]' '[:lower:]'
   ```

   Use these values to populate the frontmatter template. Never create documents with placeholder values.

4. **Generate phase research document**:
   - Use template: `@.strategic-claude-basic/templates/commands/research.template.md`
   - Replace ALL bracketed placeholders with actual metadata gathered above
   - Follow naming convention from: `@.strategic-claude-basic/research/CLAUDE.md`
   - Add phase-specific metadata to frontmatter:
     - `phase: "[Phase number and name]"`
     - `phase_deliverables: [List of key deliverables]`
     - `related_docs: ["PRD.md", "ARCHITECTURE.md", "ROADMAP.md"]`
     - `related_adrs: [List of relevant ADR numbers]`
   - Include relevant ADR references in findings sections
   - Write document to: `.strategic-claude-basic/research/[filename]` using the naming convention rules
   - Update the `@.strategic-claude-basic/research/CLAUDE.md` file with the new document entry

### Step 5: Research Presentation and Follow-up

1. **Present phase research findings**:

   ```
   I've completed comprehensive research for Phase [X.Y]: [Phase Name]

   **Research Document**: `.strategic-claude-basic/research/[filename].md`

   **Key Findings**:
   - [Technical implementation insight with file:line reference]
   - [Architecture constraint or opportunity]
   - [Dependency or integration challenge]

   **Implementation Recommendations**:
   - [Specific approach for key deliverable]
   - [Risk mitigation strategy]
   - [Success criteria validation approach]

   **ADR Compliance**:
   - [How findings align with or are constrained by architectural decisions]
   - [Any conflicts that may require new ADRs]

   **Next Steps**:
   - [Immediate actions based on research]
   - [Additional research areas if needed]

   Would you like me to research any specific aspect in more detail?
   ```

2. **Handle follow-up questions**:
   - If the user has follow-up questions about the phase, append to the same research document
   - Update the frontmatter fields `last_updated` and `last_updated_by` to reflect the update
   - Add `last_updated_note: "Added follow-up research for [phase aspect]"` to frontmatter
   - Add a new section: `## Follow-up Research [timestamp]`
   - Spawn new sub-agents as needed for additional investigation
   - Continue updating the document with phase-specific findings

## Important Guidelines

1. **Be Phase-Focused**:

- All research should directly support the specified phase objectives
- Prioritize findings that impact phase deliverables and success criteria
- Connect technical solutions to business outcomes defined in the phase

2. **Be Comprehensive but Targeted**:

- Read ALL product documentation COMPLETELY before starting research
- Focus research on phase-specific requirements and constraints
- Include specific file paths and line numbers for implementation references
- Balance depth with relevance to phase objectives

3. **Be Validation-Oriented**:

- Map findings to phase success criteria (automated and manual)
- Identify how to validate each deliverable completion
- Highlight dependencies that could impact phase timeline
- Address risks and mitigation strategies from the roadmap

4. **Be ADR-Compliant**:

- All research findings must consider existing architectural decisions
- Flag any conflicts between phase requirements and accepted ADRs
- Reference relevant ADR numbers (ADR-NNNN) when decisions influence findings
- Note when new ADRs may be needed for phase-specific architectural decisions

4. **Be Implementation-Ready**:

- Provide actionable insights for phase implementation
- Include code examples and patterns from existing codebase
- Identify reusable components and utilities
- Suggest specific technical approaches based on architecture

## Phase Research Focus Areas

### For Foundation Phases (typically 1.x):

- Infrastructure setup and core architecture
- Development environment and tooling
- Basic functionality and proof of concept
- Integration foundations and API design

### For Development Phases (typically 2.x):

- Feature implementation and business logic
- User interface and experience components
- Data management and persistence
- Integration with external services

### For Enhancement Phases (typically 3.x):

- Performance optimization and scalability
- Advanced features and edge cases
- Quality assurance and testing
- Production readiness and deployment

### For Maintenance Phases (typically 4.x+):

- Bug fixes and stability improvements
- Documentation and knowledge transfer
- Long-term maintenance planning
- Future evolution preparation

## Success Criteria for Phase Research

### Research Quality Standards:

- [ ] All phase deliverables addressed in research
- [ ] Technical implementation approaches identified
- [ ] Success criteria validation methods defined
- [ ] Dependencies and risks analyzed
- [ ] Code examples and patterns referenced
- [ ] Integration points documented
- [ ] Timeline and resource implications considered

### Documentation Standards:

- [ ] Research document follows template format
- [ ] Phase context included in frontmatter
- [ ] Findings structured around phase deliverables
- [ ] Specific file:line references provided
- [ ] Implementation recommendations included
- [ ] Next steps clearly defined

## Common Phase Research Patterns

### For New Feature Development:

- Research existing similar features
- Identify reusable components and patterns
- Plan data model and API changes
- Consider user experience and interface needs

### For Infrastructure Changes:

- Analyze current architecture constraints
- Research scalability and performance implications
- Plan migration strategies and rollback procedures
- Consider operational and maintenance impacts

### For Integration Projects:

- Research external service capabilities and constraints
- Plan authentication and authorization approaches
- Design error handling and resilience patterns
- Consider data synchronization and consistency needs

## Example Research Questions by Phase Type

### Foundation Phase Questions:

- What architecture patterns exist in the current codebase?
- How should new components integrate with existing systems?
- What development and deployment infrastructure is needed?
- What foundational utilities and libraries should be established?

### Feature Development Phase Questions:

- How do existing similar features work in the codebase?
- What data models and API changes are required?
- How should the user interface integrate with existing patterns?
- What testing and validation approaches should be used?

### Enhancement Phase Questions:

- What performance bottlenecks exist in current implementation?
- How can existing features be optimized or extended?
- What additional integrations or capabilities are needed?
- How should backward compatibility be maintained?

---

**Research Focus**: Phase-specific implementation support and validation
**Output**: Comprehensive research document aligned with phase objectives and deliverables
**Success Measure**: Research directly enables phase implementation and success criteria validation

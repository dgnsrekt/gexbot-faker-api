---
description: "Create comprehensive product documentation (PRD, ARCHITECTURE, ROADMAP, REFERENCES) based on research and requirements analysis"
argument-hint: <research_file_1> [research_file_2] [additional_research_files] [optional_context]
allowed-tools: Read(./**), Write(./strategic-claude-basic/product/**), Task, Bash(mkdir:*), Glob
model: claude-opus-4-5
---

You are tasked with creating comprehensive product documentation through an interactive, iterative process. You should be thorough, strategic, and work collaboratively with the user to produce high-quality product specifications covering requirements, architecture, roadmap, and implementation references.

**Input research files:** $1

## Initial Response

When this command is invoked:

1. **Check if research files were provided**:

   - If research file paths were provided as parameters, skip the default message
   - Immediately read any provided research files FULLY
   - Begin the product analysis process

2. **If no research files provided**, respond with:

```
I'll help you create comprehensive product documentation including PRD, ARCHITECTURE, ROADMAP, and REFERENCES files. Let me start by understanding your product vision.

Please provide:
1. Research document references (or describe the product concept)
2. Any existing requirements, user feedback, or market analysis
3. Technical constraints or architectural preferences
4. Timeline expectations and business objectives

I'll analyze this information and work with you to create complete product documentation.

Example: `/create_product_roadmap .strategic-claude-basic/research/[filename1].md .strategic-claude-basic/research/[filename2].md`
```

Then wait for the user's input.

## Process Steps

### Step 1: Context Gathering & Research Analysis

1. **Read all mentioned research files immediately and FULLY**:

   - Research documents (e.g., `.strategic-claude-basic/research/[filename].md`)
   - Market analysis documents
   - User research findings
   - Technical feasibility studies
   - **IMPORTANT**: Use the Read tool WITHOUT limit/offset parameters to read entire files
   - **CRITICAL**: DO NOT spawn sub-tasks before reading these files yourself in the main context
   - **NEVER** read files partially - if a file is mentioned, read it completely

2. **Spawn initial research tasks to understand the codebase and domain**:
   Before asking the user any questions, use specialized agents to research in parallel:

   - Use the **codebase-locator** agent to find all files related to similar products or patterns
   - Use the **codebase-analyzer** agent to understand current system architecture and capabilities
   - Use the **codebase-pattern-finder** agent to find examples of successful product implementations

   These agents will:

   - Identify existing architectural patterns and constraints
   - Find similar feature implementations to reference
   - Understand current technology stack and capabilities
   - Return specific file:line references for reusable patterns

3. **Read all files identified by research tasks**:

   - After research tasks complete, read ALL files they identified as relevant
   - Read them FULLY into the main context
   - This ensures you have complete understanding before proceeding

4. **Analyze and synthesize product requirements**:

   - Cross-reference research findings with technical capabilities
   - Identify user needs, business goals, and technical constraints
   - Note assumptions that need validation
   - Determine scope and feasibility based on research and codebase

5. **Present informed product vision and focused questions**:

   ```
   Based on the research and codebase analysis, I understand we need to create [product summary].

   Key insights from research:
   - [User need or market opportunity with research reference]
   - [Technical capability or constraint with file:line reference]
   - [Business objective or success metric from research]

   Questions that need clarification:
   - [Strategic question that affects product scope]
   - [Technical decision that impacts architecture]
   - [Business priority that affects roadmap]
   ```

   Only ask questions that you genuinely cannot answer through research analysis.

### Step 2: Product Vision Development

After getting initial clarifications:

1. **If the user corrects any misunderstanding**:

   - DO NOT just accept the correction
   - Spawn new research tasks to verify the correct information
   - Read the specific files/research they mention
   - Only proceed once you've verified the facts yourself

2. **Create a product planning todo list** using TodoWrite to track all development tasks

3. **Spawn parallel sub-tasks for comprehensive product analysis**:

   - Create multiple Task agents to research different aspects concurrently
   - Use the right agent for each type of analysis:

   **For market and user research:**

   - Use the **codebase-pattern-finder** agent to find similar successful products or features
   - Use the **codebase-analyzer** agent to understand technical feasibility and constraints

   **For competitive and technical analysis:**

   - Use the Task Tool - To find existing documentation about similar products or features
   - Use the Task Tool - To analyze technical approaches and architectural patterns

   Each agent should:

   - Find relevant examples and patterns
   - Identify technical capabilities and constraints
   - Look for successful implementations to model after
   - Return specific file:line references
   - Understand integration points and dependencies

4. **Wait for ALL sub-tasks to complete** before proceeding

5. **Present product vision and approach options**:

   ```
   Based on comprehensive analysis, here's the product vision:

   **Market Opportunity:**
   - [Key opportunity with research backing]
   - [User need with evidence]

   **Technical Approach Options:**
   1. [Approach A] - [pros/cons with technical references]
   2. [Approach B] - [pros/cons with technical references]

   **Strategic Considerations:**
   - [Business factor affecting decisions]
   - [Technical constraint or opportunity]

   Which approach aligns best with your strategic goals?
   ```

### Step 3: Documentation Structure Planning

Once aligned on product vision:

1. **Create initial documentation outline**:

   ```
   Here's my proposed documentation structure:

   ## PRD (Product Requirements Document):
   - Executive summary and problem statement
   - User stories and acceptance criteria
   - Technical and business requirements

   ## ARCHITECTURE:
   - System design and technical approach
   - Component architecture and data flow
   - Performance and security considerations

   ## ROADMAP:
   - Phased implementation with milestones
   - Timeline and resource requirements
   - Success metrics and risk management

   ## REFERENCES (if needed):
   - Local codebase references and patterns
   - Implementation examples and utilities

   Does this structure cover your needs? Should I adjust the scope or focus?
   ```

2. **Get feedback on structure** before writing detailed documentation

### Step 4: Comprehensive Documentation Generation

After structure approval:

1. **Generate all product documents:**
   - Use templates from:
     - `@.strategic-claude-basic/templates/commands/PRD.template.md`
     - `@.strategic-claude-basic/templates/commands/ARCHITECTURE.template.md`
     - `@.strategic-claude-basic/templates/commands/ROADMAP.template.md`
     - `@.strategic-claude-basic/templates/commands/REFERENCES.template.md` (only if local repos are referenced)
   - Replace ALL bracketed placeholders with actual details from research and analysis
   - Write documents to: `@.strategic-claude-basic/product/[filename].md`
   - Create REFERENCES.md only if there are local repository references needed

### Step 5: Review and Iteration

1. **Present the complete documentation suite**:

```

I've created comprehensive product documentation:

**Files created:**
- `.strategic-claude-basic/product/PRD.md` - Complete product requirements
- `.strategic-claude-basic/product/ARCHITECTURE.md` - Technical design and approach
- `.strategic-claude-basic/product/ROADMAP.md` - Implementation timeline and milestones
- `.strategic-claude-basic/product/REFERENCES.md` - Local codebase references (if applicable)

Please review the documentation and let me know:

- Are the requirements comprehensive and accurate?
- Does the technical approach align with your architecture preferences?
- Are the roadmap phases and timelines realistic?
- Any missing considerations or adjustments needed?

```

2. **Iterate based on feedback** - be ready to:

- Adjust product scope and requirements
- Refine technical architecture approach
- Modify roadmap phases and timelines
- Add/remove features or considerations
- Update success criteria and metrics

3. **Continue refining** until the user is satisfied with all documents

## Important Guidelines

1. **Be Strategic**:

- Think in terms of market opportunity and user value
- Consider business impact and competitive advantage
- Balance ambition with technical feasibility
- Focus on measurable outcomes

2. **Be Comprehensive**:

- Read all research files COMPLETELY before planning
- Research actual technical capabilities using parallel sub-tasks & sub-agents
- Review all relevant ADRs and ensure architectural compliance
- Include specific file paths and line numbers for technical references
- Create detailed, actionable documentation with clear success criteria

3. **Be ADR-Compliant**:

- All product documentation must align with accepted architectural decisions
- Flag any conflicts between product requirements and existing ADRs
- Reference relevant ADR numbers (ADR-NNNN) when decisions influence product direction
- Note when new ADRs may be needed for product-specific architectural decisions

4. **Be Collaborative**:

- Don't create all documentation in one shot
- Get buy-in at each major step
- Allow course corrections and refinements
- Work iteratively with user feedback

5. **Be Practical**:

- Ground recommendations in actual codebase capabilities
- Consider resource constraints and timelines
- Include realistic implementation phases
- Address risks and mitigation strategies

6. **Track Progress**:

- Use TodoWrite to track documentation creation tasks
- Update todos as you complete research and analysis
- Mark documentation tasks complete when done

7. **Maintain Quality Standards**:

- All documents must be production-ready quality
- Include measurable success criteria
- Provide specific technical references and examples
- Ensure consistency across all documentation

## Success Criteria Guidelines

**For each document, separate success criteria into categories:**

1. **Automated Verification** (can be validated programmatically):

- Performance benchmarks that can be measured
- Technical milestones that can be tested
- Code quality metrics that can be automated
- Integration tests that can be run

2. **Manual Verification** (requires human validation):

- User experience quality and satisfaction
- Market fit and competitive positioning
- Stakeholder alignment and buy-in
- Strategic goal achievement

## Common Documentation Patterns

### For New Product Development:

- Start with market analysis and user research
- Define clear value proposition and success metrics
- Plan technical architecture that scales
- Create phased roadmap with measurable milestones

### For Feature Enhancement:

- Document current state and limitations
- Research successful patterns in similar products
- Plan incremental improvements with clear ROI
- Include migration strategy and backward compatibility

### For Platform/Infrastructure:

- Document technical requirements and constraints
- Plan for scalability and maintainability
- Include security and performance considerations
- Create comprehensive architecture documentation

## Sub-task Spawning Best Practices

When spawning research sub-tasks:

1. **Spawn multiple tasks in parallel** for efficiency
2. **Each task should be focused** on a specific domain area
3. **Provide detailed instructions** including:
   - Exactly what to research and analyze
   - Which directories or domains to focus on
   - What information to extract and synthesize
   - Expected output format and detail level
4. **Be specific about scope**:
   - Include the full context in your prompts
5. **Specify read-only tools** to use
6. **Request specific file:line references** in responses
7. **Wait for all tasks to complete** before synthesizing
8. **Verify sub-task results**:
   - If a sub-task returns unexpected results, spawn follow-up tasks
   - Cross-check findings against actual codebase
   - Don't accept results that seem incomplete or incorrect

## Example Interaction Flow

```
User: /create_product_roadmap .strategic-claude-basic/research/[filename1].md .strategic-claude-basic/research/[filename2].md
Assistant: I'll help you create comprehensive product documentation based on your research. Let me read those research files first...

[Reads both research files fully]

Based on the market analysis and user interviews, I understand we need to create [product summary]. Let me now research the technical capabilities...

[Spawns research agents to analyze codebase and existing patterns]

[After research completes]

Based on comprehensive analysis, here's what I found:

**Market Opportunity**: [Insights from research with references]
**Technical Feasibility**: [Capabilities found in codebase]
**User Needs**: [Key findings from interviews]

Questions for strategic alignment:
- [Strategic question based on research]
- [Technical priority question]

[Interactive process continues through vision → structure → documentation → review]
```

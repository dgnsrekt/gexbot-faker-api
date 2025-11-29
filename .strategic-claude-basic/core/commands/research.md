---
description: "Conduct research on a topic and generate documentation following project standards"
argument-hint: <topic> [--with-codex] [--greenfield] [optional_description]
allowed-tools: Read(./**), Write(./strategic-claude-basic/research/**), Task, Bash(mkdir:*), Glob
model: claude-opus-4-5
---

You are tasked with conducting comprehensive research across the codebase to answer user questions by spawning parallel sub-agents and synthesizing their findings.

**Topic provided:** $1

## Flag Parsing

Check if flags are present in the provided arguments:

- If the arguments contain '--with-codex', set CODEX_RESEARCH=true for later use
- If the arguments contain '--greenfield', set GREENFIELD_MODE=true for later use
- Parse the remaining arguments as the research topic (strip flags from topic processing)
- The '--with-codex' flag enables additional research using the codex-researcher agent alongside standard agents
- The '--greenfield' flag skips codebase research agents for greenfield projects (new projects without existing codebase)

## Research Process

If no topic was provided or if you need to specify what to research, please tell me:
**What topic or area would you like me to research?**

### Steps to follow after receiving the research query:

1. **Read any directly mentioned files first:**

   - If the user mentions specific files (tickets, docs, JSON), read them FULLY first
   - **IMPORTANT**: Use the Read tool WITHOUT limit/offset parameters to read entire files
   - **CRITICAL**: Read these files yourself in the main context before spawning any sub-tasks
   - This ensures you have full context before decomposing the research

2. **Analyze and decompose the research question:**

   - Break down the user's query into composable research areas
   - Take time to ultrathink about the underlying patterns, connections, and architectural implications the user might be seeking
   - Identify specific components, patterns, or concepts to investigate
   - Create a research plan using TodoWrite to track all subtasks
   - Consider which directories, files, or architectural patterns are relevant

### Step 2.5: Architecture Decision Records (ADR) Review

1. **Discover and read all relevant ADRs**:

   - Use Glob to find all ADR files: `.strategic-claude-basic/decisions/ADR_*.md`
   - Read all ADRs with status: accepted, proposed (skip rejected, superseded)
   - Extract key decisions, rationale, and consequences that may impact research
   - Note any decisions that directly affect the research topic or approach

2. **If ADRs are found**, analyze their relevance:

   ```
   Found [N] Architecture Decision Records:
   - ADR-NNNN: [Title] - [Status] - [Relevance to research topic]
   - ADR-NNNN: [Title] - [Status] - [Relevance to research topic]

   These architectural decisions will inform the research approach and findings.
   ```

3. **If no ADRs found**, note this for context:

   ```
   No Architecture Decision Records found in .strategic-claude-basic/decisions/
   Research will proceed without architectural decision constraints.
   ```

4. **Spawn parallel sub-agent tasks for comprehensive research:**

   - Create multiple Task agents to research different aspects concurrently
   - We now have specialized agents that know how to do specific research tasks:

   **For codebase research (SKIP if GREENFIELD_MODE=true):**

   - Use the **codebase-locator** agent to find WHERE files and components live
   - Use the **codebase-analyzer** agent to understand HOW specific code works
   - Use the **codebase-pattern-finder** agent if you need examples of similar implementations

   **For web research (only if user explicitly asks OR if GREENFIELD_MODE=true):**

   - Use the **web-search-researcher** agent for external documentation and resources
   - IF you use web-research agents, instruct them to return LINKS with their findings, and please INCLUDE those links in your final report
   - In greenfield mode, focus on best practices, patterns, and external documentation for the research topic

   **For comprehensive research (when --with-codex flag is used):**

   - Use the **codex-researcher** agent for an alternative comprehensive research perspective
   - This agent leverages Codex's capabilities for both codebase and web research
   - Create a separate research document for Codex findings (see step 7 for dual document creation)
   - The codex-researcher provides a different analytical approach that may uncover additional insights
   - In greenfield mode, the codex researcher will focus on external research and best practices

   The key is to use these agents intelligently:

   - Start with locator agents to find what exists
   - Then use analyzer agents on the most promising findings
   - Run multiple agents in parallel when they're searching for different things
   - Each agent knows its job - just tell it what you're looking for
   - Don't write detailed prompts about HOW to search - the agents already know

5. **Wait for all sub-agents to complete and synthesize findings:**

   - IMPORTANT: Wait for ALL sub-agent tasks to complete before proceeding
   - Compile all sub-agent results (both codebase and external research findings)
   - **If --with-codex was used**: Prepare Codex research findings for separate document creation
   - **If GREENFIELD_MODE=true**: Note in synthesis that codebase research was intentionally skipped for greenfield project
   - Prioritize live codebase findings as primary source of truth for main document (unless in greenfield mode)
   - In greenfield mode, focus on external best practices, patterns, and recommendations
   - Note any unique insights from Codex research for the companion document
   - Connect findings across different components (or external patterns in greenfield mode)
   - Include specific file paths and line numbers for reference (when not in greenfield mode)
   - Highlight patterns, connections, and architectural decisions
   - Answer the user's specific questions with concrete evidence or external research

6. **Gather metadata for research document:**
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

7. **Generate research document(s):**

   - Use template: `@.strategic-claude-basic/templates/commands/research.template.md`
   - Replace ALL bracketed placeholders with actual metadata gathered in step 6
   - Follow naming convention from: `@.strategic-claude-basic/research/CLAUDE.md`
   - Include relevant ADR references in frontmatter and findings sections

   **If --with-codex flag was NOT used:**

   - Write single document to: `.strategic-claude-basic/research/[filename]` using the naming convention rules

   **If --with-codex flag was used:**

   - Create TWO documents with sequential numbering:
     1. **Main document**: `RESEARCH_NNNN_DD-MM-YYYY_day_subject.md` (standard research findings)
     2. **Codex document**: Use next sequential number (e.g., if main is RESEARCH_0005, use RESEARCH_0006): `RESEARCH_NNNN_DD-MM-YYYY_day_subject-codex.md` (Codex research findings)

   **Example**: If creating documents on 19-09-2025-fri about "authentication patterns":

   - Main document: `RESEARCH_0005_19-09-2025_fri_authentication-patterns.md`
   - Codex document: `RESEARCH_0006_19-09-2025_fri_authentication-patterns-codex.md`

   - Both documents should have complete frontmatter with same metadata except for the topic field:
     - Main document topic: "[Original Research Topic]"
     - Codex document topic: "[Original Research Topic] - Codex Analysis"
   - Main document contains standard research findings without Codex content
   - Codex document contains only Codex research findings with clear attribution

   **Note**: After creating both documents, update the "Next document number" in `@.strategic-claude-basic/research/CLAUDE.md` to reflect that both sequential numbers were used.

   - Update the `@.strategic-claude-basic/research/CLAUDE.md` file with BOTH document entries

8. **Present findings:**

   - Present a concise summary of findings to the user
   - Include key file references for easy navigation
   - Reference any relevant ADRs that influenced or constrain the research findings
   - Ask if they have follow-up questions or need clarification

9. **Handle follow-up questions:**
   - If the user has follow-up questions, append to the same research document
   - Update the frontmatter fields `last_updated` and `last_updated_by` to reflect the update
   - Add `last_updated_note: "Added follow-up research for [brief description]"` to frontmatter
   - Add a new section: `## Follow-up Research [timestamp]`
   - Spawn new sub-agents as needed for additional investigation
   - Continue updating the document

## Important notes:

- Always use parallel Task agents to maximize efficiency and minimize context usage
- Always run fresh codebase research - never rely solely on existing research documents (unless in greenfield mode)
- The @.strategic-claude-basic/research/ directory provides historical context to supplement live findings
- Review and consider all relevant ADRs to ensure research aligns with architectural decisions
- Focus on finding concrete file paths and line numbers for developer reference (when not in greenfield mode)
- Research documents should be self-contained with all necessary context
- Each sub-agent prompt should be specific and focused on read-only operations
- Consider cross-component connections and architectural patterns
- Include temporal context (when the research was conducted)
- Keep the main agent focused on synthesis, not deep file reading
- Encourage sub-agents to find examples and usage patterns, not just definitions
- **File reading**: Always read mentioned files FULLY (no limit/offset) before spawning sub-tasks
- **Critical ordering**: Follow the numbered steps exactly
  - ALWAYS read mentioned files first before spawning sub-tasks (step 1)
  - ALWAYS wait for all sub-agents to complete before synthesizing (step 4)
  - ALWAYS gather metadata before writing the document (step 5 before step 6)
  - NEVER write the research document with placeholder values
- **Frontmatter consistency**:
  - Always include frontmatter at the beginning of research documents
  - Keep frontmatter fields consistent across all research documents
  - Update frontmatter when adding follow-up research
  - Use snake_case for multi-word field names (e.g., `last_updated`, `git_commit`)
  - Tags should be relevant to the research topic and components studied

## Greenfield Mode Usage

The `--greenfield` flag is designed for researching topics for new projects without an existing codebase:

- **When to use**: Starting a new project, evaluating technologies for greenfield development, researching best practices without codebase constraints
- **What it does**: Skips all codebase research agents (codebase-locator, codebase-analyzer, codebase-pattern-finder) and focuses on external documentation and best practices
- **Research focus**: Industry standards, best practices, popular patterns, external documentation, and recommendations from the broader development community
- **Output**: Research documents will emphasize external resources, links, and general recommendations rather than codebase-specific findings

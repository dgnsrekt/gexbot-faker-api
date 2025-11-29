# Architecture Decision Record (ADR) Documentation Standards

> Claude Code Memory for .strategic-claude-basic/decisions directory
> Architecture Decision Record (ADR) Documentation

## Document Naming Convention

**CRITICAL**: All ADR documents in this directory MUST follow this exact pattern:

```
ADR_NNNN_DD-MM-YYYY_day_subject.md
```

Where:

- **NNNN** = Sequential ADR number (0001, 0002, etc.)
- **DD-MM-YYYY** = Date in day-month-year format
- **day** = Truncated day name (mon, tue, wed, thu, fri, sat, sun)
- **subject** = Brief hyphenated description of the decision

### ADR Status Management

Each ADR must track its status through its lifecycle:

- **proposed** - Decision is under consideration
- **accepted** - Decision has been approved and is active
- **rejected** - Decision was considered but not approved
- **deprecated** - Decision is no longer recommended but not superseded
- **superseded** - Decision has been replaced by a newer ADR

### ADR Relationship Tracking

ADRs can reference and relate to other documentation:

**Superseding Relationships**:

- Use `supersedes: [ADR-XXXX, ADR-YYYY]` for decisions that replace previous ones
- Use `superseded_by: [ADR-ZZZZ]` when this decision is replaced

**Related Documentation**:

- Reference related plans with plan document numbers
- Reference related research documents
- Reference related issues or validation documents

## Current Status

✅ All files follow the truncated day name convention.
✅ ADR template created and integrated with system.

## Existing Documents

1. **document-name** - **subject**

Next ADR number: **0001**

## ADR Lifecycle Management

### Creating New ADRs

1. **Start in "proposed" status** - All new ADRs begin as proposals
2. **Use sequential numbering** - ADR_0001, ADR_0002, etc.
3. **Include decision context** - Why this decision is needed
4. **Document alternatives** - What options were considered
5. **Define consequences** - What becomes easier/harder

### Updating ADR Status

When ADR status changes:

1. **Update the status field** in both frontmatter and body
2. **Add superseding references** if replacing previous ADRs
3. **Update this CLAUDE.md file** to reflect new ADR status
4. **Create new ADR** rather than editing accepted decisions

### ADR Numbering Scheme

- **ADR-0001** to **ADR-0999**: General architectural decisions
- **ADR-1000** to **ADR-1999**: Infrastructure and tooling decisions
- **ADR-2000** to **ADR-2999**: Security and compliance decisions
- **ADR-3000** to **ADR-3999**: Performance and scalability decisions
- **ADR-4000** to **ADR-4999**: Integration and API decisions

Use the general range (0001-0999) unless the decision clearly fits a specific category.

## Connection to Other Documentation

### Relationship to Plans

- ADRs often result from implementation plans
- Reference related PLAN documents in the "Related Decisions" section
- Include plan document number when applicable

### Relationship to Research

- ADRs should reference supporting research documents
- Include RESEARCH document numbers that informed the decision
- Cross-reference technical analysis that supports the decision

### Relationship to Issues

- ADRs may address problems identified in ISSUE documents
- Reference related issues in the "Context" section
- Include issue document numbers when applicable

### Relationship to Validation

- Decisions may require validation through VALIDATION documents
- Include validation criteria in ADR implementation details
- Reference validation documents once created

## Instructions for Claude Code

When working in this directory:

1. **Always check file names** against the convention before creating new files
2. **Refuse to create** files that don't follow the naming pattern
3. **Suggest correct names** when violations are detected
4. **Maintain sequential numbering** for new ADRs
5. **Use truncated day names** (mon, tue, wed, thu, fri, sat, sun)
6. **Get current date dynamically**: Run `date '+%d-%m-%Y-%a' | tr '[:upper:]' '[:lower:]'` to get properly formatted date (e.g., "17-09-2025-tue")
7. **Start all ADRs as "proposed"** - Do not create ADRs in "accepted" status
8. **Use the ADR template** from `.strategic-claude-basic/templates/commands/adr.template.md`
9. **Update this file**: After creating new ADRs, update the "Existing Documents" list and "Next ADR number"
10. **Check for related documentation** - Look for related plans, research, issues before creating ADRs
11. **Maintain decision traceability** - Include references to supporting documentation

### ADR Creation Workflow

1. **Research phase**: Check for related plans, research, issues
2. **Template usage**: Use the adr.template.md for consistent structure
3. **Numbering**: Use next sequential ADR number
4. **Status**: Always start as "proposed"
5. **Review**: Ensure all required sections are completed
6. **Documentation update**: Update this CLAUDE.md file

## Quality Standards

### Required ADR Content

- **Clear problem statement** - What decision needs to be made
- **Comprehensive context** - Background and driving factors
- **Alternatives analysis** - Options considered with pros/cons
- **Decision rationale** - Why this option was chosen
- **Implementation approach** - How the decision will be executed
- **Consequences assessment** - Positive and negative impacts
- **Success criteria** - How to measure decision effectiveness

### Documentation Quality

- **Professional presentation** - Proper formatting and structure
- **Technical accuracy** - Correct implementation details
- **Actionable content** - Specific, measurable criteria
- **Cross-references** - Links to related documentation
- **Code references** - File paths and line numbers when applicable

## Benefits of This Convention

This naming and management system ensures:

- **Decision traceability** - Clear record of architectural decisions over time
- **Status transparency** - Easy tracking of decision lifecycle
- **Relationship mapping** - Clear connections between related decisions
- **Historical context** - Chronological ordering shows decision evolution
- **Documentation integration** - Seamless connection with plans, research, issues
- **Consistent standards** - Uniform approach across all architectural documentation
- **Search and discovery** - Easy finding of decisions by number, date, or topic
- **Change management** - Clear process for updating and superseding decisions

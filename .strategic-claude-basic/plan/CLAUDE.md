# Plan Documentation Standards

> Claude Code Memory for .strategic-claude-basic/plan directory
> Plan Documentation

## Document Naming Convention

**CRITICAL**: All plan documents in this directory MUST follow these exact patterns:

**Implementation Plans:**
```
PLAN_NNNN_DD-MM-YYYY_day_subject.md
```

**Test Plans:**
```
TEST_NNNN_DD-MM-YYYY_day_subject.md
```

Where:

- **NNNN** = Sequential document number (0001, 0002, etc.) - shared numbering sequence
- **DD-MM-YYYY** = Date in day-month-year format
- **day** = Truncated day name (mon, tue, wed, thu, fri, sat, sun)
- **subject** = Brief hyphenated description (same for related implementation and test plans)

### Subject Naming for Phase-Specific Plans

When using the `/plan_phase` command, use these subject patterns:

**For specific milestones**:

- `phase1-1-project-infrastructure` (Milestone 1.1)
- `phase2-3-motion-blur-integration` (Milestone 2.3)
- `phase3-2-visual-theme-system` (Milestone 3.2)

**For comprehensive plans**:

- `phase1-foundation-implementation` (All Phase 1 milestones)
- `phase2-performance-enhancement` (All Phase 2 milestones)

This ensures phase-specific plans are easily identifiable and searchable.

## Current Status

✅ All files now follow the truncated day name convention.

## Existing Documents

1. **document-name** - **subject**

Next document number: **0001**

## Plan Type Guidelines

### Implementation Plans (PLAN_):
- Focus on building and architecture
- Implementation phases and technical approach
- Integration points and dependencies
- Minimal testing sections (reference test plan)

### Test Plans (TEST_):
- Focus on validation and quality assurance
- Test coverage strategy and test types
- Test data management and CI integration
- Cross-reference related implementation plan

### Related Plan Linking:
- Implementation and test plans for the same feature share the same subject name
- Cross-reference each other in the "References" sections
- Maintain logical numbering (e.g., PLAN_0001 and TEST_0001 for the same feature)

## Instructions for Claude Code

When working in this directory:

1. **Always check file names** against the convention before creating new files
2. **Refuse to create** files that don't follow the naming pattern
3. **Suggest correct names** when violations are detected
4. **Maintain sequential numbering** for new documents (shared sequence for PLAN_ and TEST_)
5. **Use truncated day names** (mon, tue, wed, thu, fri, sat, sun)
6. **Get current date dynamically**: Run `date '+%d-%m-%Y-%a' | tr '[:upper:]' '[:lower:]'` to get properly formatted date (e.g., "16-08-2025-sat")
7. **Create paired plans when tests are needed**: Both PLAN_ and TEST_ documents for complex features
8. **Update this file**: After creating new documents, update the "Existing Documents" list and "Next document number" in this CLAUDE.md file

This naming convention ensures:

- Chronological ordering of plan documents showing evolution of the project
- Clear identification of document purpose and creation date
- Consistent documentation standards across the project
- Historical context for architectural decisions and research findings

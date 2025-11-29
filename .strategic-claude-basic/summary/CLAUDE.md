# Summary Documentation Standards

> Claude Code Memory for .strategic-claude-basic/summary directory
> Summary Documentation

## Document Naming Convention

**CRITICAL**: All summary documents in this directory MUST follow this exact pattern:

**When connected to a PLAN document**:

```
SUMMARY_PLAN_NNNN_DD-MM-YYYY_day_subject.md
```

**When connected to a TEST document**:

```
SUMMARY_TEST_NNNN_DD-MM-YYYY_day_subject.md
```

**When no plan exists**:

```
SUMMARY_DD-MM-YYYY_day_subject.md
```

Where:

- **NNNN** = Sequential document number matching the associated plan (0001, 0002, etc.)
- **DD-MM-YYYY** = Date in day-month-year format
- **day** = Truncated day name (mon, tue, wed, thu, fri, sat, sun)
- **subject** = Brief hyphenated description matching the plan subject

### Plan Connection Rules

**Primary Rule**: Summaries should ALWAYS be connected to a plan document when possible.

- Use the EXACT same NNNN number as the associated plan
- Use the EXACT same subject as the associated plan
- Include the plan type (PLAN or TEST) in the summary name
- Match the date format and day name convention

**Examples**:

- Plan: `PLAN_0001_07-09-2025_sat_user-authentication-system.md`
- Summary: `SUMMARY_PLAN_0001_07-09-2025_sat_user-authentication-system.md`

- Test Plan: `TEST_0001_07-09-2025_sat_user-authentication-system.md`
- Summary: `SUMMARY_TEST_0001_07-09-2025_sat_user-authentication-system.md`

**Only omit NNNN** when explicitly creating a summary with no associated plan document.

## Current Status

✅ All files follow the truncated day name convention.
✅ Plan-connected summaries use matching document numbers.

## Existing Documents

1. **document-name** - **subject**

Next document number: **0001** (or match existing plan number)

## Instructions for Claude Code

When working in this directory:

1. **Check for associated plan first** - Look in `.strategic-claude-basic/plan/` for matching plan document
2. **Match plan naming exactly** - Use same NNNN, date format, and subject as the plan
3. **Always check file names** against the convention before creating new files
4. **Refuse to create** files that don't follow the naming pattern
5. **Suggest correct names** when violations are detected
6. **Use truncated day names** (mon, tue, wed, thu, fri, sat, sun)
7. **Get current date dynamically**: Run `date '+%d-%m-%Y-%a' | tr '[:upper:]' '[:lower:]'` to get properly formatted date (e.g., "07-09-2025-sat")
8. **Update this file**: After creating new documents, update the "Existing Documents" list

This naming convention ensures:

- Clear connection between summaries and their associated plans
- Prevention of overwriting between PLAN and TEST summaries
- Distinct tracking of implementation vs. test work
- Chronological ordering of summary documents
- Consistent documentation standards across the project
- Easy traceability from implementation results back to original plans

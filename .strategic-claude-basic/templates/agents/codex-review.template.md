## Code Review Summary

[Brief overview of the code changes reviewed and overall assessment]

## Critical Findings (P0-P1)

### [P0] [Critical Issue Title]
**File**: `path/to/file.ext:line-range`
**Impact**: [Immediate blocking issue description]
**Details**: [Clear explanation of the problem and why it's critical]
**Confidence**: [0.0-1.0]

### [P1] [Urgent Issue Title]
**File**: `path/to/file.ext:line-range`
**Impact**: [Urgent issue requiring next cycle attention]
**Details**: [Problem description and implications]
**Confidence**: [0.0-1.0]

## Standard Findings (P2)

### [P2] [Normal Priority Issue Title]
**File**: `path/to/file.ext:line-range`
**Impact**: [Normal priority issue for eventual resolution]
**Details**: [Problem description and recommended fix]
**Confidence**: [0.0-1.0]

## Improvement Suggestions (P3)

### [P3] [Low Priority Suggestion Title]
**File**: `path/to/file.ext:line-range`
**Impact**: [Nice-to-have improvement]
**Details**: [Suggestion for enhancement]
**Confidence**: [0.0-1.0]

## Security Analysis

### Security Findings
- **[Severity]** [Security issue description] (`file:line`)
- **Assessment**: [Security posture evaluation]

### Security Recommendations
- [Security improvement suggestion 1]
- [Security improvement suggestion 2]

## Performance Analysis

### Performance Findings
- **[Impact Level]** [Performance issue description] (`file:line`)
- **Assessment**: [Performance impact evaluation]

### Performance Recommendations
- [Performance optimization suggestion 1]
- [Performance optimization suggestion 2]

## Code Quality Assessment

### Maintainability
- **Readability**: [Assessment of code clarity and documentation]
- **Structure**: [Evaluation of organization and architecture]
- **Conventions**: [Adherence to established patterns and standards]

### Test Coverage
- **Coverage Assessment**: [Analysis of test completeness]
- **Testing Recommendations**: [Suggestions for improved testing]

## Overall Assessment

### Correctness Verdict
**Status**: [Patch is correct | Patch is incorrect]

**Reasoning**: [1-3 sentence explanation justifying the verdict]

**Confidence Score**: [0.0-1.0]

### Priority Summary
- **P0 Issues**: [Count] - [Brief description if any]
- **P1 Issues**: [Count] - [Brief description if any]
- **P2 Issues**: [Count] - [Brief description if any]
- **P3 Issues**: [Count] - [Brief description if any]

### Recommendations

#### Immediate Actions Required
- [Action 1 for P0/P1 issues]
- [Action 2 for P0/P1 issues]

#### Future Improvements
- [Improvement 1 for P2/P3 issues]
- [Improvement 2 for P2/P3 issues]

## Structured Output (JSON Format)

```json
{
  "findings": [
    {
      "title": "≤ 80 chars, imperative description",
      "body": "Valid Markdown explaining why this is a problem; cite files/lines/functions",
      "confidence_score": 0.0-1.0,
      "priority": 0-3,
      "code_location": {
        "absolute_file_path": "file path",
        "line_range": {"start": int, "end": int}
      }
    }
  ],
  "overall_correctness": "patch is correct | patch is incorrect",
  "overall_explanation": "1-3 sentence explanation justifying the overall_correctness verdict",
  "overall_confidence_score": 0.0-1.0
}
```

## Review Metadata

- **Review completed**: [Timestamp]
- **Files analyzed**: [Number and list of files reviewed]
- **Lines of code reviewed**: [Approximate count]
- **Review depth**: [Surface level | Standard | Deep analysis]
- **External references consulted**: [Any documentation or standards referenced]

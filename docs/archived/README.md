# Archived planning docs — historical, NOT current

⚠️ **Agents & humans: do not treat anything in this directory as current.**
These files describe intent and state *at the time they were written*
(late Nov 2025). The code has moved since. Verify against the actual source
before relying on any claim here.

## Where these came from

This repo was originally scaffolded with the **Strategic Claude Basic**
harness (`web-explorer` template), which drove work through a
`research → plan → execute → summarize` loop and wrote its output under
`.strategic-claude-basic/{plan,research,summary}/`.

That harness has been removed from the project. These 7 markdown files were
the only *project-specific* knowledge it produced, so they were moved here
instead of being deleted:

- `PLAN_0001…` gexbot historical downloader
- `PLAN_0002…` gex faker api v2
- `PLAN_0003…` websocket orderflow hub
- `RESEARCH_0001…` quant historical analysis
- `SUMMARY_PLAN_0001…0003` summaries of the above plans

## Why keep them

Design rationale and context that isn't captured in the code or commit
messages. Useful as background reading — treat as a snapshot, not a spec.
For current behavior, read the code, `README.md`, and `CLAUDE.md`.

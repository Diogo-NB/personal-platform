# Repository agent instructions

## Code comment policy

- Prefer code that explains itself through precise names, small focused units,
  and clear control flow.
- Add comments only for warnings, non-obvious constraints, rationale, or
  behavior that the code cannot express clearly on its own. Explain why the
  code exists or what risk it prevents, not what each statement does.
- Do not add comments that narrate identifiers, signatures, control flow, or
  otherwise repeat information already explicit in the code.
- Before adding an explanatory comment, first try to make the code clearer.
  Remove redundant or stale comments from code you touch.
- Preserve required machine directives, build tags, license headers, generated
  code markers, and justified linter suppressions.

## Scoped project guidance

- Before reviewing or modifying anything under `tools/skycrate/`, read and follow
  `tools/skycrate/AGENTS.md`.
- Before reviewing or modifying anything under `apps/ts6/` or
  `infra/aws/teamspeak6/`, read and follow `apps/ts6/AGENTS.md`.
- Before reviewing or modifying anything under `apps/ts6-management-api/`, read
  and follow `apps/ts6-management-api/AGENTS.md`.
- Before reviewing or modifying anything under `apps/ts6-management-web/`, read
  and follow `apps/ts6-management-web/AGENTS.md`.
- Before reviewing or modifying anything under `apps/dns-updater/`, read and
  follow `apps/dns-updater/AGENTS.md`.

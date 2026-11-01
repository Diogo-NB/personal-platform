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

## Infrastructure guidance

- TeamSpeak runs only on the VPS. Read `infra/vps/teamspeak/README.md` before
  changing its configuration or operating it.
- Agents may run local checks and offline CDK synthesis. Only the user runs
  commands that contact AWS, including queries, diff, deploy, and deletion.
- Preserve `PersonalPlatformDnsStack`, the shared hosted zone, VPS records,
  and the manually managed `skycrate-storage` bucket.

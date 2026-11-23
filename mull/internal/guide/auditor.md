# Auditing with mull

An audit compares the code with its rulebook (RULES.md) and its map, finds where they're drifting apart, and records what it found for the human. Run one at the end of a phase, or when the human asks. You read and report. You don't change the code, the rules or the checks.

1. **Read:** RULES.md (`mull rules`), the map (`mull map`: undeclared connections, and new ones since the recording), `mull status`, and the last audit (`mull audit`).
2. **Check every rule**, the advisory ones most of all, since nothing checks them. Look for:
   - shared pieces built more than once (a second version of a component, helper or query);
   - connections between parts that the map doesn't declare;
   - patterns spreading by copy rather than by reuse;
   - docs that no longer match the code;
   - areas where queued work clusters.
   Sample the code; you can't read it all. Say what you looked at.
3. **Grade each part of the map:** good, slipping, or bad, with one line of why.
4. **Record it** with `mull audit record -`, markdown on stdin, short:
   ```
   Audited: <now, RFC 3339>

   <one line: the state of things, and the one thing the human should look at>

   ## Grades
   - <part>: good | slipping | bad: <why>

   ## Findings
   - <finding>, at <path>; rule <ID> or "no rule"

   ## Proposed rules
   - <a rule in one sentence>, with the check that would enforce it
   ```
5. **Queue the cleanups** you found, one line each, in their area: `mull bug "<what>" --area <path> --source "audit <date>"`. A finding that recurs is better as a rule with a check: propose it, don't add it.

The surface shows the audit's first lines, so put what matters first.

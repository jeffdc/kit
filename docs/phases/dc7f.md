7. **Shared areas and the integration review gate.**
   - Overlap is derived from git: for each pair of items in flight, the files both branches changed (claim base to branch tip) show as a problem in the view.
   - A plan's rules may list `shared` globs, and `done` flags changes to them in the report.
   - A phase may require an integration review (`mull phase set <id> integration-review on`, Jeff's call). Once all its items are done, it shows "awaiting integration review" instead of complete, and phases that need it stay blocked.
   - The supervisor, in a worktree on the target, runs `mull done <phase-id> --summary --tour --design -`. That runs the checks on the target tip, writes a report with the phase's combined diff, and files a review entry on the phase.
   - Accepting that review completes the phase. `do --reason` creates a committed fix-up item in the phase, which reopens it.

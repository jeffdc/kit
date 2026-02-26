---
name: finishing-a-development-branch
description: Use when implementation is complete, all tests pass, and you need to decide how to integrate the work — merge, PR, or keep the branch
---

# Finishing a Development Branch

## Overview

Verify tests pass, present integration options, execute the chosen workflow.

**Core principle:** Verify first, then present clear options. Don't assume the workflow.

## Process

### Step 1: Verify Tests

Run the project's test suite before presenting any options.

```bash
# Use whatever the project needs
mix test / go test ./... / npm test / pytest
```

**If tests fail:** Report the failures. Do not proceed to Step 2. Fix first.

**If tests pass:** Continue.

### Step 2: Determine Base Branch

```bash
git merge-base HEAD main 2>/dev/null || git merge-base HEAD master 2>/dev/null
```

Or ask: "This branch split from main — is that correct?"

### Step 3: Present Options

```
Implementation complete. Tests passing. What would you like to do?

1. Merge back to <base-branch> locally
2. Push and create a Pull Request
3. Keep the branch as-is (I'll handle it later)

Which option?
```

Keep it concise. Don't add explanations.

### Step 4: Execute Choice

#### Option 1: Merge Locally

```bash
git checkout <base-branch>
git pull
git merge <feature-branch>
```

Run tests again on the merged result to verify nothing broke.

Report: "Merged `<feature-branch>` into `<base-branch>`. Tests passing. Branch `<feature-branch>` still exists if you want to delete it."

Don't delete the branch automatically.

#### Option 2: Push and Create PR

```bash
git push -u origin <feature-branch>

gh pr create --title "<title>" --body "$(cat <<'EOF'
## Summary
<2-3 bullets of what changed>

## Test plan
- [ ] <verification steps>
EOF
)"
```

Report the PR URL.

#### Option 3: Keep As-Is

Report: "Keeping branch `<feature-branch>` as-is. You can come back to it later."

Done.

## Red Flags

**Never:**
- Proceed with failing tests
- Merge without verifying tests on the result
- Delete branches without the user asking
- Force-push without explicit request

**Always:**
- Verify tests before offering options
- Present exactly 3 options
- Run tests again after merge (Option 1)

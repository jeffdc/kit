#!/bin/sh
# Creates the demo repository make dev serves, if it isn't there: a scratch
# git repo on mull with something in every part of the surface, for trying
# the UI by hand or from a headless browser. Usage: dev-demo.sh <dev dir>
# <mull>. Everything lives under <dev dir>; its config dir keeps the
# registry and last-looked state away from the real ones.
set -eu
dev=$1 mull=$2
demo=$dev/demo
[ -d "$demo/.git" ] && exit 0

mkdir -p "$demo" "$dev/cfg"
export XDG_CONFIG_HOME="$dev/cfg"
cd "$demo"
git init -q -b main
m() { "$mull" "$@" >/dev/null; }
id() { "$mull" "$@" | sed -n 's/^  "id": "\([0-9a-f]*\)".*/\1/p' | head -1; }
# work <id> <file> <content> <message> [done flags…]: starts the item in
# its own worktree (the runner makes it), commits one file there and runs
# done, from that worktree.
work() {
	item=$1 file=$2 content=$3 msg=$4
	shift 4
	wt=$("$mull" start "$item" | sed -n 's/.*"worktree": "\([^"]*\)".*/\1/p' | head -1)
	mkdir -p "$wt/$(dirname "$file")"
	printf '%s\n' "$content" >"$wt/$file"
	git -C "$wt" add "$file"
	git -C "$wt" commit -q -m "$msg" -m "Mull: $item"
	(cd "$wt" && "$mull" done "$item" "$@" >/dev/null)
}

# The rulebook: a ratcheted check (no new TODOs in the app; one known), a
# plain check, and two rules with no check (advisory).
mkdir -p app scripts .mull/ratchets
cat >RULES.md <<'EOF'
# Rules

## R1: Pages use the shared components

- scope: app/**
- check: scripts/check-todo
- ratchet: yes

A page that builds its own widgets drifts from the rest; each TODO marks one.

## R2: No debug logs in the app

- scope: app/**
- check: test ! -e app/debug.log

## R3: Keep each page under 300 lines

- scope: app/**

A hope until someone writes the check.

## R4: Migrations can be rolled back

- scope: db/**
EOF
printf '#!/bin/sh\ngrep -rl TODO app 2>/dev/null | sort\nexit 0\n' >scripts/check-todo
chmod +x scripts/check-todo
echo "<!-- TODO: use the shared header -->" >app/legacy.html
echo "app/legacy.html" >.mull/ratchets/R1.txt
git add RULES.md app scripts .mull
git commit -q -m "demo"

m init
echo '{"op": "config checks", "commands": ["true"], "on-request": true}' | "$mull" batch >/dev/null
# Migrations are high risk; over 30 lines is medium; no low-risk sample
# yet (one is drawn below, on purpose).
m config risk --add "db/**" --max-lines 30 --no-sample --on-request

m plan new web "Web app" --summary "Search, mail and the account pages." --on-request
m plan agree web --on-request
m plan set web doc - <<'EOF'
The web app's search, mail and account pages. Search comes first because mail links into it.
EOF
search=$(id phase add "Search" --plan web --summary "Find anything from the top bar.")
mailp=$(id phase add "Mail" --plan web --summary "Send and read mail.")
account=$(id phase add "Account" --plan web --summary "Settings and invites.")
billing=$(id phase add "Billing" --plan web --summary "Cards and invoices.")
launch=$(id phase add "Launch" --plan web --summary "Turn the pages on for everyone.")
m phase set "$launch" needs "$mailp"
# Billing has work cut but nothing started: it's still uphill.
m add "Card payments" --phase "$billing"

# Before the last look: search landed, with the rulebook's ratchet at one.
results=$(id add "Search results page" --phase "$search")
work "$results" app/search.html "<main>results</main>" "Search results page" --summary "A results page under the top bar." --rules none

# The last look is here: what follows is new since.
sleep 1
tip=$(git rev-parse mull)
mkdir -p "$dev/cfg/mull"
printf '{"repos": {"%s": {"commit": "%s", "at": "%s"}}}\n' "$demo" "$tip" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >"$dev/cfg/mull/seen.json"
sleep 1

# High risk: a migration waits for review.
invites=$(id add "Invites table" --phase "$account")
work "$invites" db/001_invites.sql "create table invites (id int);" "Invites table" --summary "A table for pending invites, with its migration." --rules none
# Medium: a big template lands, and is shown.
templates=$(id add "Mail templates" --phase "$mailp")
work "$templates" app/mail/templates.html "$(seq 1 40 | sed 's/.*/<p>line &<\/p>/')" "Mail templates" --summary "Forty templates for transactional mail." --rules none --slowed "The template fixtures took a while to generate"
# Low, sampled: it lands, and a sample is drawn for a look.
m config risk --sample 100 --on-request
filters=$(id add "Search filters" --phase "$search")
work "$filters" app/filters.html "<aside>filters</aside>" "Search filters" --summary "Filter results by date and author." --rules none
m config risk --no-sample --on-request
# Unplanned work: landed at low risk, not sampled, so it's only in Flow.
login=$(id add "Fix the login redirect" --plan none)
work "$login" app/login.html "<form>login</form>" "Fix the login redirect" --summary "The redirect after login keeps the page you came from." --rules none
# A new TODO breaks the ratchet: done fails, and R1 is failing (1 → 2).
# It's the last done, as the surface counts each rule from the latest run.
inbox=$(id add "Mail inbox" --phase "$mailp")
work "$inbox" app/mail/inbox.html "<!-- TODO: shared list --><main>inbox</main>" "Mail inbox" --summary "The inbox list." --rules none 2>/dev/null || true

# Questions: one with time left on its default, one lapsed.
compose=$(id add "Compose mail" --phase "$mailp")
m ask "Which queue do mails go through?" --item "$compose" --default "The existing Redis queue" --by 2d
m ask "Keep the v1 search API?" --plan web --default "Keep it for a release, then remove it" --by 1s
sleep 2

# The agents' queue: three bugs in one area (a cluster), and two more.
m bug "Results jump when filters load" --plan web --area "app/search/**"
m bug "Empty search shows a spinner forever" --plan web --area "app/search/**"
m bug "Search box loses focus on back" --plan web --area "app/search/**"
m bug "The footer overlaps the last result on small screens" --plan web --source "demo"
m idea "Saved searches" --plan web

# A plan still shaping, for Agree this plan.
m plan new pilot "Pilot with two teams" --summary "Try the web app with two teams before launch." --on-request
echo "demo repo: $demo"

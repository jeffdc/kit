#!/bin/sh
# Creates the demo repository make dev serves, if it isn't there: a scratch
# git repo on mull with something in every Needs you tier, for trying the UI
# by hand or from a headless browser. Usage: dev-demo.sh <dev dir> <mull>.
# Everything lives under <dev dir>; its config dir keeps the registry and
# last-looked state away from the real ones.
set -eu
dev=$1 mull=$2
demo=$dev/demo
[ -d "$demo/.git" ] && exit 0

mkdir -p "$demo" "$dev/cfg"
export XDG_CONFIG_HOME="$dev/cfg"
cd "$demo"
git init -q -b main
git commit -q --allow-empty -m "demo"
m() { "$mull" "$@" >/dev/null; }
id() { "$mull" "$@" | sed -n 's/^  "id": "\([0-9a-f]*\)".*/\1/p' | head -1; }

m init
echo '{"op": "config checks", "commands": ["true"]}' | "$mull" batch >/dev/null
m plan new web "Web app" --summary "Search, mail and the account pages." --on-request
m plan agree web --on-request
m plan doc web - <<'EOF'
The web app's search, mail and account pages. This doc is long enough to show how prose runs on the sheet, with the status margin beside it: search comes first because mail links into it.
EOF

# A ready item with a question on it: the Blocking tier.
mail=$(id add "Mailer" --plan web)
m ask "Which queue do mails go through?" --item "$mail"
# Decisions from an item, and from a planning session: the Decisions tier.
search=$(id add "Search page" --plan web)
m decided "Search runs on Postgres full-text, not a separate index" --item "$search"
m decided "Results page 25 per page" --item "$search"
m decided "Keep the API on /v2" --plan web --source "planning call"
# An item in review carrying decisions: the Review tier and a packet.
digest=$(id add "Digest job" --plan web)
m start "$digest"
m decided "Digests skip users with no activity" --item "$digest"
m decided "One digest per timezone batch" --item "$digest"
m done "$digest" --summary "A nightly digest job." --tour none --design "A cron job reading the activity table."
# A long title, and a captured bug for the backlog.
m add "Drop or park a Needs you entry with a reason" --plan web
m bug "The footer overlaps the last result on small screens" --plan web --source "demo"
echo "demo repo: $demo"

# Queue and asks (draft for oaks)

## Bugs: stay queued, with an area

| ID | Bug | Area | Where |
|---|---|---|---|
| b558 | Malformed stored synonyms | lib/oaks/species | Taxonomy |
| cb93 | Nine hybrids' synonyms joined by = | lib/oaks/species | Taxonomy |
| a946 | Nine hybrids' taxonomy disagrees with their parents' | lib/oaks/species | Taxonomy |
| d2a4 | Species page: title row, duplicate Edit pairs, tab strip | lib/oaks_web/live/species_detail_live.ex | Taxon pages |
| 180f | IUCN stamp has no source mark | lib/oaks_web/components/design_components.ex | Taxon pages |
| 4742 | Search broken | lib/oaks/search.ex | search |
| d1d7 | 404 flood writes a row per request | lib/oaks_web/plugs/analytics.ex | unplanned |
| 0e05 | Login-limit test fails on a repeated run | test/oaks_web/controllers | unplanned |
| f5a5 | Markdown keeps `class`, so content can pull in any class | lib/oaks/markdown.ex | unplanned |
| c804 | build_external_links still copies logos | lib/oaks_web/live/species_detail_live.ex | unplanned |
| cc6f | URLs in JSON link fields aren't validated (was an idea) | lib/oaks/changeset_helpers.ex | unplanned |
| 440a | Seven test files copy ConnCase's login helper (was an idea) | test/support | unplanned |
| new | Check the CSP on prod in a browser for violations (was question 948c) | lib/oaks_web/plugs | unplanned |

Dropped: cab5 (page CSS restyles components), now R17's ratchet; ba9a (roman names in an article), folded into ask 1. Committed fd65 (sandbox log noise) stays unplanned.

## Ideas: to plan shaping

- Taxonomy: ed7a. Taxon pages: 334d, 45f6 (moved from unplanned), a41d, 4533. ID tool: c948, a903. Search: fa45 (moved).
- No plan yet, kept for later: 03c1 passkeys, 5cd4 disease tolerance, 8ef1 USDA guide, e3f0 iOS app.

## Asks

1. **Two fixes to your article "How to Document an Oak":** a `- ` before "vestiture" (review 3ff5) and `*…*` around its scientific names (bug ba9a). They change no words, but it's your text. Default: you make them in the article editor; no agent touches it. By 2026-12-07.
2. **Publishing (re-asks 7570):** does marking a section written publish it, or do sections wait for a Publish? Default: marking it written publishes it. By 2026-12-07, before the Authoring editor starts.
3. **Entry structure (re-asks da8b):** the sections after key sentence, account and historical context. Default: those three for now, more added as you need them. By 2026-12-07.

Settled by the plans, not asks: c0eb (where infraspecific rank is stored: the Taxonomy phase's taxa model), e821 and 3bff (search's shaping questions, now in its doc), 948c (a task, above).

## The 12 Polish reviews

All passed their checks, are small (11 to 48 lines) and are already on main.

**Accept in one batch (9):** 4593, 8a38, 873e, e7cc, 49ff, c097, ec11, 4057, 3ff5. Accepting ratifies the 5 agent decisions they carry (e57b, 7a54, 09fe, d902, 33d7): 48px list rows on a subgrid; long authorities end in an ellipsis; compare heads link to the source's page; About's statistics as a sentence; only the three settings pages get Back to settings. c097 and 3ff5 changed no code (one no longer reproduces; one is ask 1).

**Worth a look (3), each high risk under the new bands:**
- **c0d4:** changes `lib/oaks_web/user_auth.ex` (the setup-complete page now gets the new user's scope). Auth code.
- **3fa9:** puts `put_secure_browser_headers` back in the router's browser pipeline, fed the CSP from SecurityHeaders, so Sobelow passes. Security headers.
- **0edd:** adds a rule to DESIGN.md, the Field Face Rule (prose fields are multi-line serif; every other field is one line in Archivo). No fields changed.

After a look: `mull accept c0d4 3fa9 0edd --on-request`.

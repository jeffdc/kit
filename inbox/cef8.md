---
kind: decision
text: The fonts are Google Fonts' variable woff2 builds (Archivo wdth 62–125 wght 100–900, Red Hat Mono wght 300–700), latin and latin-ext subsets only, font-display swap, with the latin files preloaded
plan: mull-serve
item: "0114"
by: /Users/jeff/dev/kit
created: "2026-11-11T01:15:45Z"
---

One file per subset covers every width and weight the CSS uses (~210 KB); Vietnamese isn't needed; anything outside the subsets falls back to the system faces

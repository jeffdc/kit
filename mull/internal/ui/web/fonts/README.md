# Fonts

Self-hosted so `mull serve` never calls out. Both are under the SIL Open Font License 1.1 (`*-OFL.txt`).

| file | family | axes | source |
|---|---|---|---|
| `archivo-latin.woff2` | Archivo | wdth 62–125, wght 100–900 | Google Fonts, archivo v25, latin subset |
| `archivo-latin-ext.woff2` | Archivo | same | archivo v25, latin-ext subset |
| `red-hat-mono-latin.woff2` | Red Hat Mono | wght 300–700 | Google Fonts, redhatmono v16, latin subset |
| `red-hat-mono-latin-ext.woff2` | Red Hat Mono | same | redhatmono v16, latin-ext subset |

Fetched 2026-11-11 from the CSS that `https://fonts.googleapis.com/css2?family=Archivo:wdth,wght@62..125,100..900&family=Red+Hat+Mono:wght@300..700` serves to a current browser; the unicode ranges in `style.css` are copied from it. Licences from `github.com/google/fonts` (`ofl/archivo`, `ofl/redhatmono`). Glyphs outside these subsets fall back to the system faces in `--font` and `--mono`.

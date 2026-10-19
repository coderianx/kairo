# Vendored UI libraries

Kairo's renderer loads plain scripts (no bundler); these browser builds are
checked in so the CSP (`default-src 'self'`) stays strict — no remote code.

| File | Library | Version | Source | License |
| --- | --- | --- | --- | --- |
| `marked.min.js` | [marked](https://github.com/markedjs/marked) | 12.0.2 | `https://cdn.jsdelivr.net/npm/marked@12.0.2/marked.min.js` | MIT |
| `purify.min.js` | [DOMPurify](https://github.com/cure53/DOMPurify) | 3.1.6 | `https://cdn.jsdelivr.net/npm/dompurify@3.1.6/dist/purify.min.js` | Apache-2.0 / MPL-2.0 |

To update a library, download the new UMD/minified build over the existing
file and bump the version here. The renderer degrades to plain escaped text
automatically if either file fails to load.

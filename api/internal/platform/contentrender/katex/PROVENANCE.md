# Vendored KaTeX runtime

- Component: `katex`
- Version: `0.16.44`
- Upstream: <https://github.com/KaTeX/KaTeX>
- Source file: `website/node_modules/katex/dist/katex.min.js`
- Source lock: `website/package-lock.json`
- SHA-256: `781acf302f284be795aea416fe3566a98a8db8023fa6a61cacea5cd4715bef51`
- License: MIT, preserved as `assets/LICENSE.katex`

`assets/katex.min.js` is the exact browser bundle already locked and installed by
the Judgify frontend. It is embedded into the Go API so authored-content
rendering does not need Node.js, a subprocess, a CDN, or network access at
runtime.

When upgrading, update the frontend lock first, mechanically copy the bundle
and license again, then update both the expected SHA-256 and runtime version.
The constructor deliberately refuses to start when either check differs.

## Authored-math security dialect

The API intentionally supports the presentation-focused KaTeX subset needed by
problem statements and other authored content. Before JavaScript execution it
rejects trust/HTML/embed commands, user macro definitions and aliases, direct
private `@` macros, raw `\raisebox`, and direct font-sizing controls. Those
surfaces can create state, active content, inline styles, or disproportionate
transient/layout work and are not required for semantic DSA notation.

After rendering, the engine validates the exact pinned `(element, attribute)`
MathML dimension grammar, nested scale, per-dimension magnitude, aggregate
dimension work, and token count. Any mismatch rejects the complete expression;
the outer HTML sanitizer independently repeats the element-scoped allowlist as
defense in depth. A KaTeX upgrade must refresh both the compatibility corpus and
this reviewed output policy before the renderer version is advanced.

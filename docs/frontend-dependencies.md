# Frontend dependency overrides

`frontend/package.json` pins the transitive `happy-dom` dependency to **21.0.4** to fix [GHSA-834v-8h66-3h79](https://github.com/advisories/GHSA-834v-8h66-3h79). Puck 0.23.0 still requests `^20.0.10`, so a normal lockfile update cannot select the patched major version. The override also covers Tiptap's copy.

Happy DOM 21 disables CSS and iframe loading by default. Kinlight does not rely on those server-side loading features. Before removing the override, confirm that Puck's supported dependency range selects a patched release, run `npm ci` and `npm audit`, and check Arrange plus a Tiptap server HTML round-trip.

The lockfile also updates Zod from 4.6.5 to **4.6.7**, within TanStack's existing version range, to fix [GHSA-mrhx-c7xc-m3w5](https://github.com/advisories/GHSA-mrhx-c7xc-m3w5). No override is needed for Zod.

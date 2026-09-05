# Homepage design QA

final result: passed

## Evidence

- Source: `C:/Users/qianl/.codex/generated_images/01a070ef-7b7f-7481-bb87-592dec7347a0/exec-dc8549e4-197b-4ef7-a59e-5adb217a3326.png`.
- Browser: `http://127.0.0.1:5173/`, English, light, scroll position 0.
- Captures: `C:/Users/qianl/.codex/visualizations/2026/09/05/01a070ef-7b7f-7481-bb87-592dec7347a0/` (`wox-home-desktop.png`, `wox-home-mobile.png`, `wox-home-zh-dark.png`, `wox-system-plugins.png`).
- Desktop CSS viewport: 1421 × 1107, device pixel ratio approximately 1. Source and implementation shown together in the same comparison tool output; no density rescaling required. Mobile: 390 × 844.
- User-approved deviations: Reddit, GitHub and Discord replace the hero installation link; system plugins use a compact linked list instead of a screenshot carousel. Existing documentation navigation and theme toggle remain available.

## Comparison and fixes

1. Extra document heading borders and paragraph margins shifted the screenshot down. Removed the inherited heading borders/padding and specified hero paragraph margins. Recompared the captured desktop against the source.
2. Mobile inherited both document and homepage gutters, causing social buttons to wrap onto two rows. Removed duplicate document gutters; recaptured mobile with all three community buttons in one row and no horizontal overflow. Added the missing word space when the desktop line break is hidden.
3. Set an opaque homepage navigation background so scrolled content cannot show through it.

## Fidelity review

- Typography: retained the existing sans-serif stack; 52px desktop heading with the same two-line structure, readable 18px introductory text and 15px controls. Mobile headline wraps naturally.
- Layout: centered hero, 650px screenshot, restrained section separators; screenshot and section start align closely with the selected target after margin corrections.
- Colors: neutral website surfaces, text and buttons in light and dark modes. Product screenshots retain their colors. Inverse text colors also applied to store primary buttons to avoid white-on-white dark-mode controls.
- Imagery: original glass-theme screenshot edited using the built-in image tool to remove desktop and glance. The result is RGB, not transparent; CSS provides rounded clipping and shadow. Small source text was visually reviewed in the full-resolution asset. This is an edited raster, not a pixel-identical native capture.
- Copy: hero follows the selected draft; English and Chinese are updated together. Lower-page copy is shorter and linked to existing guides.

## Verification

- `pnpm docs:build` and `git diff --check` passed.
- Verified rendered hero destinations for download, Reddit, GitHub and Discord. External services were not logged into or otherwise changed.
- Clicked the File item and verified navigation to its documentation.
- Checked English mobile and Chinese mobile for overflow; no broken images on the English homepage.
- Tested Chinese light/dark switching. Browser warning/error log was empty.
- Keyboard focus remains visible on functional links; nine system-plugin guide links render.

No remaining actionable P0/P1/P2 findings in the changed surface. The theme carousel was subsequently removed at the user's request. Its section now contains localized text and a theme-store link; browser verification found zero images and zero switching buttons in that section. The updated build passed. A complete store-flow or screen-reader audit is outside this check.

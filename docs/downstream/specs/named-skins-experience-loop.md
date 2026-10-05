# Spec: Named Skins Experience And Quality Loop

Status: `baseline-shipped`

Follow-up: [Named Skins Confidence And Recovery Loop](./named-skins-confidence-and-recovery-loop.md)

> Historical baseline: the problem and solution below describe the pre-implementation state and the contract that was delivered. Use the follow-up spec for current gaps.

## Problem Statement

Multica already offers Tension, Relay, and Field skins with light, dark, and system appearance. The controls, persistence, semantic tokens, reduced-motion behavior, and multiple platform implementations are substantial, but the feature is maintained as a broad set of visual edits rather than a small cross-platform product contract. Preferences are primarily device-local, visual coverage can drift across Web, Desktop, Docs, and Mobile, and upstream shell changes can repeatedly reopen raw-color, contrast, hydration, and selected-state regressions.

For users, a skin is valuable only when it is reliable everywhere they work. A preference that flashes the wrong appearance at startup, fails to follow the account across devices, makes a status ambiguous, or produces unreadable content is worse than no personalization. For maintainers, every additional direct surface edit increases downstream merge cost without proving product value.

## Solution

Turn named skins into a coherent appearance preference and quality system. Keep exactly the existing three skins and make them conform to one versioned semantic-token contract. Authenticated application preferences will sync across devices while retaining immediate offline-safe local application. Web, Desktop, and Mobile will implement one appearance preference interface through platform adapters; Docs will use the same token contract with local anonymous persistence.

The feature will be considered complete only when startup is flicker-free, every state remains legible without color alone, all skin/mode combinations meet contrast requirements, keyboard and reduced-motion behavior are consistent, and automated visual/contract checks prevent drift. Product analytics will measure adoption and sync reliability without collecting content.

## User Stories

1. As a user, I want to choose Tension, Relay, or Field, so that the interface matches my visual preference.
2. As a user, I want skin and light/dark/system appearance to remain independent, so that changing ambient mode does not change the product's visual identity.
3. As a signed-in user, I want my skin and appearance choice to follow me across Web, Desktop, and Mobile, so that Multica feels consistent on every device.
4. As an offline user, I want my last known preference to apply immediately, so that the app remains usable without waiting for the server.
5. As a user changing preference offline, I want the choice to sync when connectivity returns, so that local use is not discarded.
6. As a user, I want startup to render the correct appearance before first paint, so that I do not see a flash of the default skin.
7. As a system-mode user, I want Multica to follow operating-system light/dark changes without rewriting my preference, so that automatic appearance remains predictable.
8. As a user, I want a clear selected state and realistic preview for every skin, so that I understand the choice before applying it.
9. As a user, I want a one-action reset to product defaults, so that experimentation is reversible.
10. As a keyboard user, I want arrow-key navigation, visible focus, selection, and reset behavior in appearance settings, so that the feature is fully operable.
11. As a screen-reader user, I want every option to expose its name, description, and selected state, so that visual previews have a nonvisual equivalent.
12. As a user with reduced motion enabled, I want skin and appearance changes to happen without reveal animation, so that personalization does not cause discomfort.
13. As a user in a browser without View Transitions, I want an immediate stable switch, so that unsupported animation never blocks the change.
14. As a user, I want status, priority, destructive, success, warning, info, selection, and focus semantics to remain recognizable in every skin, so that appearance does not alter meaning.
15. As a color-vision-deficient user, I want critical state to use text, icon, shape, or weight in addition to color, so that the interface remains understandable.
16. As a low-vision user, I want WCAG 2.2 AA contrast and visible focus in every supported skin/mode combination, so that content remains readable.
17. As a Windows high-contrast user, I want controls and status identity to remain visible in forced-colors mode, so that system accessibility settings are respected.
18. As a user at 200% zoom, I want appearance controls and dense operational pages to remain usable without overlap or clipped labels, so that zoom does not break workflows.
19. As a CJK-language user, I want labels and descriptions to wrap naturally, so that translated appearance settings remain polished.
20. As a user, I want charts, Markdown, code, editor content, dialogs, popovers, and loading/error/empty states to use the active semantic appearance, so that no raw-color islands remain.
21. As a Desktop user, I want window chrome and renderer content to agree at launch and during switching, so that the application feels native and cohesive.
22. As a Mobile user, I want native controls, safe areas, keyboards, and sheets to use the same semantics without imitating desktop layout, so that consistency does not sacrifice platform fit.
23. As a Docs reader, I want the same named skins with local persistence and no account requirement, so that documentation matches the product while respecting anonymous use.
24. As a product operator, I want to know skin adoption, reset rate, sync failures, and appearance-related errors, so that maintenance is guided by evidence.
25. As a privacy-conscious user, I want appearance analytics to exclude content and browsing behavior, so that personalization does not become tracking.
26. As a maintainer, I want a missing semantic token or raw product color to fail automated checks, so that drift is caught before release.
27. As a maintainer, I want upstream pages to inherit tokens without downstream markup rewrites, so that future upstream merges stay small.
28. As a maintainer, I want a versioned token contract with migration guidance, so that intentional design changes remain coordinated across platforms.
29. As a support engineer, I want diagnostics to report resolved skin, requested appearance, source of preference, and sync status without personal content, so that appearance bugs are diagnosable.
30. As a release owner, I want representative visual evidence for every skin and mode on every application platform, so that releases do not rely on subjective spot checks.

## Implementation Decisions

- Keep exactly the existing skin identifiers `tension`, `relay`, and `field`, with `tension` as the default. Skin and appearance remain separate preferences; appearance values remain `system`, `light`, and `dark`.
- Define one versioned `AppearancePreferences` domain contract containing requested skin, requested appearance, resolved appearance, preference source, updated timestamp, sync state, and token-contract version.
- Place the external seam at an `AppearancePreferenceAdapter` interface used by shared settings and bootstrap logic. Real adapters exist for Web, Desktop, Mobile, and anonymous Docs, so the seam represents actual platform variation.
- Shared core logic validates identifiers, resolves defaults and system mode, reconciles local/server timestamps, and returns state. It does not access browser storage, native storage, process environment, or UI libraries.
- Authenticated user preferences persist server-side and sync across Web, Desktop, and Mobile. Each platform also keeps an immediate local cache for startup and offline use. Anonymous Docs remains local-only.
- Reconciliation uses last explicit user change, not the currently resolved system appearance. A system light/dark event updates only the resolved appearance and never creates a server write.
- Preference writes apply locally first, then sync through the existing authenticated user-preference mutation path. Failed sync remains visible and retryable without reverting a valid local choice.
- Web bootstrap uses server-rendered preference metadata or a pre-paint bootstrap value to set skin and appearance before hydration. Desktop preload and Mobile startup provide equivalent pre-render values through their platform adapters.
- Define one semantic token schema covering canvas, shell, surface, raised surface, hover, selection, borders, text tiers, focus, brand, destructive, success, warning, info, status identities, charts, editor, code, overlays, and platform chrome.
- Every skin implements the full schema in light and dark modes. `system` resolves to one of those modes and does not create a third color set.
- Product surfaces consume only semantic tokens. Introduce automated lint/check tooling that rejects unapproved raw colors, missing tokens, token cycles, and direct skin-name branching in product modules.
- Keep platform-native layout and interaction implementations. Shared semantics do not require Mobile to copy Web/Desktop markup or interaction density.
- Preserve View Transition reveal motion as an enhancement for explicit user changes. It must be skipped for reduced motion, system-driven mode changes, unsupported environments, background tabs, and startup.
- Selected and active states remain identifiable during hover and without color through at least one stable additional dimension such as weight, icon, outline, or label.
- Add forced-colors/high-contrast overrides at semantic-token and control levels rather than per-page patches.
- Add a diagnostics snapshot with requested/resolved appearance, skin, adapter source, token version, reduced-motion state, forced-colors state, and last sync error class. It contains no route history or user content.
- Add bounded analytics events for appearance viewed, skin selected, appearance selected, reset, sync failure/recovery, and invalid stored value recovery. Do not include free text, route content, workspace identifiers, or document content.
- Treat raw-color islands in landing pages and Docs as migration debt. Convert them when touched and track the remaining count through automated checks; do not rewrite upstream page structure only to migrate color usage.
- Keep design changes in token definitions, shared primitives, or narrow wrappers. Do not restyle upstream-owned onboarding, authentication, Issue, or navigation shells in this work.
- Add no further skins in this spec. New skin proposals require adoption evidence, complete token coverage, visual evidence, and an explicit maintenance-cost decision.
- Roll out server sync additively. Existing local preferences are imported on first authenticated sync only when no newer server preference exists, preventing the default from overwriting an intentional choice.
- Any schema/index work follows repository rules: no foreign keys/cascades, explicit cleanup, and standalone concurrent-index migrations.

## Testing Decisions

- The primary behavioral seam is the `AppearancePreferenceAdapter` contract exercised with Web, Desktop, Mobile, and Docs adapters. The same conformance suite verifies load, validate, apply, persist, offline change, reconnect, timestamp conflict, invalid value recovery, system-mode resolution, reset, and diagnostics.
- The canonical user-flow E2E uses two authenticated browser contexts plus a Desktop adapter fixture: change skin and appearance, verify immediate application and server sync, open a second context, resolve a simulated offline conflict, follow a system-mode change, reset, and confirm no startup flash or hydration mismatch.
- Keep pure preference parsing/reconciliation tests in a node environment. Do not repeat the complete state matrix through mounted settings views.
- Extend existing appearance preference, settings keyboard, text-contrast, and theme provider tests as prior art rather than introducing parallel suites.
- Add a token-contract checker that loads every skin/light-dark combination and fails on missing keys, invalid references, raw values outside approved token sources, and semantic status collisions.
- Add automated contrast tests for text, focus indicators, borders required for control recognition, status graphics, charts, code, selection, destructive actions, and disabled states. Contrast tests supplement but do not replace screenshot review.
- Record screenshot acceptance evidence for the six explicit skin/mode combinations on representative operational, editor, settings, dialog/popover, public, Docs, Desktop, and Mobile surfaces. `system` is tested behaviorally against simulated OS changes rather than duplicating screenshots.
- Include narrow viewport, 200% zoom, long English and CJK labels, reduced motion, forced colors, keyboard-only operation, and screen-reader semantics in acceptance evidence.
- Assert that switching appearance does not resize fixed controls, shift page structure, lose input focus, close overlays, clear drafts, or change semantic status identity.
- Add startup performance assertions for correct pre-paint attributes, no theme hydration warning, no appearance-caused layout shift, and no blocking network dependency before the first usable render.
- Test cross-tab and cross-window storage events, signed-in server updates, stale local timestamps, failed writes, and recovery without exposing storage implementation to shared callers.
- Test analytics only at its bounded interface and assert that content, route text, workspace identifiers, and arbitrary properties cannot be supplied.
- Run shared/app typechecks, focused tests, token and contrast checks, production builds, Playwright visual/behavior flows, and Mobile preference tests before completion.

## Out of Scope

- Adding a fourth skin, a user-authored theme editor, arbitrary color pickers, or marketplace themes.
- Changing product information architecture, page composition, status meaning, typography scale, or upstream-owned shell markup solely for visual novelty.
- Synchronizing anonymous Docs preferences to an account.
- Replacing platform-native Mobile controls with shared Web markup.
- Decorative ambient animation, gradient effects, negative letter spacing, or appearance-specific business behavior.
- Collecting page content, workspace activity, or navigation history as appearance analytics.
- Claiming that skin choice by itself improves retention or revenue without measured evidence.

## Further Notes

- Success metrics: percentage of signed-in users whose explicit preference appears consistently on a second device; sync failure/recovery rate; invalid-value recovery rate; appearance-related support issues; skin adoption and 28-day persistence; zero known WCAG AA violations in the required matrix.
- Guardrails: first-paint mismatch rate, hydration warnings, appearance-caused layout shift, switch latency, visual-regression count, raw-color count, and upstream merge conflict count attributable to theme work.
- Product investment should remain capped after this spec. The intended outcome is a reliable, low-maintenance personalization system, not an expanding theme product.
- Delivery should proceed in order: domain contract and adapters; authenticated sync/bootstrap; token enforcement and debt migration; accessibility/visual matrix; diagnostics and metrics; maintenance-mode handoff.

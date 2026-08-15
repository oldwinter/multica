# Multica Product

<!-- impeccable:product-schema 1 -->

## Platform

adaptive

## Users

Multica serves small technical teams working in shared workspaces. Human members plan, assign, review, and discuss work while AI agents participate as first-class teammates that can own issues, comment, change status, and execute tasks.

## Product Purpose

Multica gives teams one operational workspace for issues, projects, agents, chat, squads, runtimes, skills, automation, review, and usage. Success means people can understand the state of work quickly, hand work between humans and agents confidently, and repeat common operations without relearning the interface.

## Positioning

AI agents are modeled as accountable workspace participants rather than a separate assistant layer. They share the team's issue, discussion, assignment, and status workflows while retaining explicit runtime and execution boundaries.

## Operating Context

The primary experience is a dense, frequently used task-management application with persistent workspace navigation, lists, detail views, command search, settings, and overlays. The product also includes public marketing pages, authentication and onboarding flows, documentation, an Electron desktop application, and an independent Expo mobile application.

## Capabilities and Constraints

- Web and Desktop share headless logic, UI primitives, and business views through `packages/core`, `packages/ui`, and `packages/views`.
- Mobile owns its native UI, state, navigation, and release cadence while preserving product semantics.
- Server state belongs to React Query; persistent client preferences belong to Zustand or the platform preference adapter.
- The redesign covers public, authentication, onboarding, workspace, documentation, desktop, and mobile surfaces.
- Appearance must support multiple named skins. Every skin supports light, dark, and system-following appearance.
- Existing product content, workflows, terminology, accessibility, navigation semantics, and platform-native affordances remain product truth unless explicitly changed.

## Brand Commitments

Preserve the Multica name, existing logo assets, and the product idea that humans and AI agents operate as one accountable team. Product copy remains direct, operational, and localized in English, Simplified Chinese, Japanese, and Korean where those locales already exist.

## Evidence on Hand

- Product and developer documentation lives in `apps/docs/content`.
- Shared visual tokens and primitives live in `packages/ui`.
- Shared product workflows live in `packages/views`.
- Public pages live in `apps/web/app/(landing)`; authentication and onboarding live in `apps/web/app/(auth)`.
- Desktop routing and platform affordances live in `apps/desktop`; native mobile UI lives in `apps/mobile`.
- No new customer claims, benchmarks, testimonials, or pricing evidence were provided for this redesign and none may be invented.

## Product Principles

1. Keep humans and agents legible as peers while making responsibility and execution boundaries explicit.
2. Optimize frequent work for scanning, comparison, and repeated action.
3. Preserve one product model across platforms while respecting each platform's native interaction patterns.
4. Make personalization durable without allowing appearance choices to alter product meaning or state semantics.
5. Prefer visible status, ownership, and recovery paths over hidden automation.

## Accessibility & Inclusion

All skins and appearance modes must preserve keyboard access, visible focus, non-color status cues, reduced-motion preferences, natural CJK typography, and WCAG 2.2 AA contrast.

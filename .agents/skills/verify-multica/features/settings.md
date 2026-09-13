# Settings

Settings is the account and workspace configuration page. A user opens it from the sidebar and switches among tabs such as General, Members, and Preferences.

## Sub-features

- `settings-open` opens Settings from the sidebar.
- `settings-general` shows the General workspace tab.
- `settings-members` shows the Members tab.
- `settings-preferences` opens Preferences (appearance) via `?tab=preferences`.

## How to get to it (user POV)

- Sidebar link `Settings`.
- URL `/{slug}/settings`.
- URL `/{slug}/settings?tab=preferences` (and other tab query values: `general`, `members`, `profile`, …).

## Driving it with control-multica

Preconditions:

- Doctor is ok.
- Disposable session at desktop width (1440px).
- Start at `/{slug}/issues`.

- **Sidebar entry.** Choose link `Settings` exact. URL matches `/settings`. Heading `Settings`. Tab `General` visible. Tab `Members` visible.
- **Members.** Choose tab `Members`. Members UI is showing (the verify user as a member is enough; do not invite anyone).
- **Preferences URL.** `goto /{slug}/settings?tab=preferences`. Text `Appearance` is visible. Radios include skins such as `Tension` / `Relay` / `Field` and modes `Dark` / system-equivalent. Do **not** change appearance on a shared human account; on a `verify-*` user, changing a radio and seeing `Synced across devices` (or the pending copy if offline) is a valid extra check — revert before cleanup if you do.
- **Proof.** Screenshots of Settings chrome and one inner tab. Artifacts: `artifacts/<run-id>/settings/`.

## Gotchas

- Appearance is account-scoped. Never click skins on `dev@localhost` in a shared profile.
- Below `md`, tabs become a select named `Settings`. Widen the viewport.
- Billing, GitHub, Plugins, and similar tabs can be flag-gated. If a tab is absent, record `verified-unreachable` with the missing tab name; do not fail Settings as a whole when General and Members still render.
- `settings-preferences` is not a proof of cross-device sync. That longer flow lives in `e2e/settings.spec.ts` and needs two contexts; skip it unless you are extending this map.

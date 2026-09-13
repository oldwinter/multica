# Sidebar navigation

The signed-in shell has a primary sidebar. Choosing Inbox, Agents, Issues, or Settings changes the route, the page heading, and the browser tab title.

## Sub-features

- `nav-inbox` opens Inbox.
- `nav-agents` opens Agents.
- `nav-issues` returns to Issues.
- `nav-settings` opens Settings with account/workspace tabs.

## How to get to it (user POV)

- Use the sidebar (navigation named `Primary navigation`).
- Direct URLs: `/{slug}/inbox`, `/{slug}/agents`, `/{slug}/issues`, `/{slug}/settings`.

## Driving it with control-multica

Preconditions:

- Doctor is ok.
- Disposable session injected; start at `/{slug}/issues` with `New Issue` visible.

- **Inbox.** Choose link `Inbox`. URL matches `/inbox`. Page text includes `Inbox`. Tab title `Inbox | Multica`. Empty state may show `No notifications` / `Your inbox is empty` — that is success, not a failure.
- **Agents.** Choose link `Agents`. URL matches `/agents`. Heading/text `Agents`. Tab title `Agents | Multica`. Button `New agent` is the create entry (do not require an existing agent).
- **Issues.** Choose link `Issues` with exact name (Chat/other items must not match). URL matches `/issues`. `New Issue` visible. Tab title `Issues | Multica`.
- **Settings.** Choose link `Settings` exact. URL matches `/settings`. Heading `Settings`. Tabs `General` and `Members` are visible on desktop width (1440px).
- **Proof.** Screenshot each destination with the tab title or heading visible. Record URLs. Artifacts: `artifacts/<run-id>/navigation/`.

## Gotchas

- `getByRole("link", { name: "Issues" })` without `exact` can hit the wrong control. Use `exact: true` for Issues and Settings.
- Viewport below `md` replaces settings tabs with a select named `Settings`. Drive at ≥1440px unless you are proving mobile web.
- Other sidebar items exist (`Chat`, `My Issues`, `Projects`, `Autopilot`, `Rooms`, `Office`, `Twin`, `Wiki`, `Squads`, `Analytics`, `Runtimes`, `Skills`). They are not this feature's required proof. If you click them, do not call this feature fully verified unless the four listed destinations also succeeded.
- Unread badges on Inbox/Chat change accessible names. Prefer role+name that still matches `Inbox` / `Chat`.

# Multica web verification map

This directory is the maintained source for verifying user-facing behavior of the Multica **web** app in this checkout. Read this index before driving, then use the matching feature file as the recipe.

Desktop, iOS, and CLI are not covered here.

## Baseline preconditions

- This checkout's environment is up (`make up` or a reused matching `make status`).
- `node .agents/skills/verify-multica/control-multica.mjs doctor` is ok.
- Drive a disposable `verify-*` user and workspace from `control-multica login`, not `dev@localhost` / slug `dev`.
- Browser locale is `en-US`. Account `language` is `en`.
- `multica:chat:isOpen` is `"false"`. Create-issue last mode is `manual`.
- Never drive an instance whose `/health` commit is a different checkout.

## Driving conventions

- Start every recipe from the issues page (`/{slug}/issues`) unless the feature's preconditions say otherwise.
- Prefer ARIA roles and accessible names. The table in `SKILL.md` is the handle list.
- Treat quoted names as literal English chrome.
- Restore or delete `verify-*` data after a mutation. Do not delete proof artifacts.

## Proof and skip reporting

- Capture the action and the resulting state, not only the last screen.
- UI proof includes a screenshot with `| Multica` in the tab or chrome, plus a text dump or ARIA snapshot of the same view.
- Mutation proof includes a second user-facing view (open the issue, reload the destination).
- Record the feature ID and entry point on every artifact (`evidence.json` does this for `drive issues`).
- An unreachable path needs the attempted URL/control and the unmet precondition. Do not report it verified via a different path.

## Feature entry contract

Each feature file starts with an H1 and one paragraph, then exactly these H2s:

1. `Sub-features`
2. `How to get to it (user POV)`
3. `Driving it with control-multica`
4. `Gotchas`

## Features

- [Sign in](./login.md) covers the email+code form, unauthenticated redirect, and log out.
- [Issues](./issues.md) covers board chrome, manual create, cancel, and detail.
- [Sidebar navigation](./navigation.md) covers Inbox, Agents, Issues, and Settings destinations.
- [Issue comments](./comments.md) covers composing and posting a comment on an issue.
- [Settings](./settings.md) covers opening Settings and the General / Members / Preferences tabs.

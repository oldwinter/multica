---
name: verify-multica
description: Drive this checkout's Multica web app like a user — launch the local API+web environment, doctor identity, exercise UI paths, and keep proof artifacts. Use when proving a Multica UI change, reproducing a web bug, or checking a user-visible flow on the real app rather than unit tests.
---

# Verify Multica (web)

This skill is for an agent that has never seen the app. Multica is an AI-native task workspace. The **primary user surface is the Next.js web app**. Desktop (Electron), iOS, the `multica` CLI, and the Go API exist; do not treat them as the default drive target. Playwright specs in `e2e/` are regression tests that assume servers are already up — they are not a substitute for this skill's launch/doctor/evidence loop.

Repo root is three directories above this file. Run every command from the repo root.

## Isolate before you touch anything

This checkout can share a machine with other Multica environments. Ports and database names are allocated into `~/.multica/dev/` and written to `.env` (main) or `.env.worktree` (linked worktree). Two instances **can** run side by side only when they have different ports and database names.

Rules:

- Drive only an API whose `GET /health` `pid` + `commit` belong to **this** checkout. `make up` refuses to reuse a foreign listener; you must too.
- Never attach to the operator's already-open browser profile. Use a fresh Playwright context (the helper) or a fresh browser tab that you authenticate yourself. Logging out of their `dev@localhost` session is a defect in the run, not a proof.
- Never create issues in the human `dev` workspace. The helper mints `verify-<run>@multica.local` and workspace slug `verify-<run>`.
- If `/health` answers on this checkout's port but `commit` does not match `git rev-parse --short HEAD`, stop. Do not kill it by process name.

## Launch

Preferred command (records identity, unique ports, seeds `dev@localhost` / code `888888` for humans — you still use a disposable verify user):

```bash
make up
```

Ready when `make status` shows api + web running for this directory, and:

- `GET http://localhost:$BACKEND_PORT/health` → `{"status":"ok","pid":...,"commit":"<short sha>","started_at":"..."}`
- `GET http://localhost:$BACKEND_PORT/healthz` → `{"status":"ok","checks":{"db":"ok","migrations":"ok"}}`
- `http://localhost:$FRONTEND_PORT/` answers

`make up` prints the origin and workspace slug (`dev` by default). Read ports from `.env.worktree` if present, else `.env`. Do not assume 8080/3000.

If this checkout already has a matching environment, **reuse it**. Set nothing; do not start a second `make up` on the same ports. Record `VERIFY_STARTED_ENV=0`.

If nothing is registered, start it and record `VERIFY_STARTED_ENV=1` in the shell that will later clean up. Agent-owned throwaway:

```bash
make up ARGS=--ephemeral
```

Teardown of what **this run started**:

```bash
make down
```

`make down` keeps the database. `make destroy` drops it — never use destroy for verification cleanup. If you reused an existing matching environment, do not `make down`.

Windows: run launch from Git Bash at the repo root (`make up` once `make` exists, or `./scripts/dev-env.sh up`). From PowerShell in the repo root, that is:

```powershell
& "C:\Program Files\Git\bin\bash.exe" ./scripts/dev-env.sh up
```

Do not start a second launcher. The Node helper does not start processes.

## Doctor

Read-only. Run first, and again after any failed drive or surprise.

```bash
node .agents/skills/verify-multica/control-multica.mjs doctor
```

Pass means: API live, db+migrations ready, frontend answering, and `/health` commit matches this checkout (or is `unknown` with an explicit warning that you started the process here). Fail means do not drive.

`make status` is the human-readable identity ledger (pid, commit, ports, database name). Prefer the helper's JSON when scripting.

## Drive

Two equivalent harnesses, same handles:

1. **Helper (preferred for scripted proofs):** Playwright, English locale, fresh storage, token injected only after API login of a disposable user.
2. **Interactive agent browser:** Cursor browser tools or any CDP. Same ARIA names. Still a fresh session.

Mint a disposable session (also writes `artifacts/<run-id>/session.json`):

```bash
node .agents/skills/verify-multica/control-multica.mjs login
```

That call: `POST /auth/send-code` → `POST /auth/verify-code` with `MULTICA_DEV_VERIFICATION_CODE` or `888888` → `PATCH /api/me` `{name, language:"en"}` → ensure workspace → `POST /api/me/onboarding/complete` `{"exit":"existing"}`. It does **not** click the login form. The login feature file is the one that drives the form.

Inject into a fresh browser context before `goto`:

- `localStorage.multica_token` = session token
- `localStorage["multica:chat:isOpen"]` = `"false"` (chat overlay otherwise covers the page)
- `localStorage.multica_create_mode` = `{"state":{"lastMode":"manual"},"version":0}` so **New Issue** opens the manual editor, not agent quick-create

Then open `/{workspace_slug}/issues`. Wait for the button named `New Issue`. Tab title for that page is `Issues | Multica`.

Stable handles (English UI; force `language: "en"` and `locale: "en-US"`):

| What | Handle |
| --- | --- |
| Login heading | text `Sign in to Multica` |
| Email | textbox named `Email` (`#login-email`) |
| Send code | button `Continue` (disabled while email empty) |
| Verify heading | `Check your email` |
| OTP | hidden textbox (`input-otp`); typing 6 digits auto-submits |
| Sidebar | navigation named `Primary navigation` |
| Issues / Inbox / Agents / Settings / Chat | links with those exact names (`Issues` needs `exact`) |
| New issue | button `New Issue` (also shortcut `C` outside inputs) |
| Issue title | textbox named `Issue title` |
| Submit issue | button `Create Issue` |
| Created | text `Issue created`, then button `View issue` |
| Detail | text `Properties`; tab title `{identifier}: {title} \| Multica` |
| Board columns | `Backlog`, `Todo`, `In Progress` |
| List / Board | buttons/text `List` / `Board` |
| Comment shell | `getByTestId("comment-composer-shell")` then ProseMirror `Leave a comment...`; submit `ControlOrMeta+Enter` |
| Log out | workspace switcher button (workspace name) → menuitem `Log out` |

Do not use test-only HTTP to assert a user-visible mutation except as a **second** read after the UI action. Setup (login, onboarding complete, extra seed rows) may use the API. Creating the issue under test in the issues feature must go through **New Issue**.

Read the feature map before driving. A proof that hits one convenient entry point is incomplete when the map lists others.

Scripted issues drive:

```bash
node .agents/skills/verify-multica/control-multica.mjs drive issues
# or, after login:
node .agents/skills/verify-multica/control-multica.mjs drive issues --run-id <run-id>
```

## Evidence

Keep proof under `.agents/skills/verify-multica/artifacts/<run-id>/`. That directory survives cleanup. It is gitignored except `.gitignore`.

A proof is not a single screenshot of the final screen. Capture:

1. The user action (modal open, click, typed title).
2. The resulting UI (toast + detail, or destination URL + heading).
3. A second view of the same fact (tab title, body text, or API list of the disposable workspace).

Standards:

- Exercise the real path. No internal store setters, no `e2e/`-only seed as the thing under test.
- Side effects: an issue that exists only in the create toast has not been proved; open it.
- Locale: artifacts must show English chrome (`New Issue`, `Properties`, `| Multica`). If you see `新建任务`, you are not on `en` — stop and fix language, do not translate the map live.
- Mocks: none on this path. Email codes in local non-production come from `MULTICA_DEV_VERIFICATION_CODE` or server stdout; that is the product's local auth, not a test double.

## Cleanup

```bash
node .agents/skills/verify-multica/control-multica.mjs cleanup --run-id <run-id>
```

Deletes issues recorded on the session, then the `verify-*` workspace, then the `verify-*` user row. Does **not** delete `artifacts/<run-id>/`.

Add `--stop-env` only when this run started the environment (`VERIFY_STARTED_ENV=1` / `session.started_env`). That runs `make down` for this checkout's registry entry, not `kill` by name.

Failed iterations: run the same cleanup with the run id you have. Leftover `verify-*` users are worse than a missing screenshot; leftover `make up` processes on a reused env are worse than both.

## Helpers

All invocations from repo root:

```bash
node .agents/skills/verify-multica/control-multica.mjs doctor
node .agents/skills/verify-multica/control-multica.mjs login
node .agents/skills/verify-multica/control-multica.mjs drive issues
node .agents/skills/verify-multica/control-multica.mjs drive issues --run-id <run-id>
node .agents/skills/verify-multica/control-multica.mjs cleanup --run-id <run-id>
node .agents/skills/verify-multica/control-multica.mjs cleanup --run-id <run-id> --stop-env
```

`login` and `drive` print `run_id`. Save it. `drive issues` writes `artifacts/<run-id>/issues/evidence.json` plus PNG/text captures.

## Other surfaces (out of scope unless the map grows)

- Desktop: `make up C=desktop`. Different userData dir per env. Not this skill's default.
- Mobile: `apps/mobile/`; independent stack. Do not import this harness there.
- CLI: `server/bin/multica` after `make up` writes a profile under the env's `PROFILE`. Separate from web proof.
- Playwright suite: `pnpm exec playwright test` — servers must already be up; it shares the database. Do not run it as a way to "start" verification, and do not run it against a workspace you are about to `destroy`.

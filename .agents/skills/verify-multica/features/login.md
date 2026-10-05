# Sign in

Sign in lets a user request a 6-digit email code, verify it, and land in the workspace; an unauthenticated visit to a protected page is sent to login; log out returns to the login screen.

## Sub-features

- `login-form` shows the email step with Continue disabled until an email is present.
- `login-code` accepts a 6-digit code and authenticates.
- `login-redirect` sends an anonymous visit to a workspace issues URL to `/login`.
- `login-logout` signs the user out from the workspace switcher.

## How to get to it (user POV)

- Open `/login`.
- Visit `/{slug}/issues` with no session and follow the redirect to `/login`.
- From a signed-in session, open the workspace switcher and choose `Log out`.

## Driving it with control-multica

Preconditions:

- Doctor is ok.
- `MULTICA_DEV_VERIFICATION_CODE` is a 6-digit value (make up writes `888888`) and `APP_ENV` is not `production`.
- Use a Playwright (or other) **fresh** context. Do not use the operator's existing tab.
- For `login-code` of a user who must skip onboarding: create them with `control-multica login` in this run, then in a fresh browser context drive `/login` with that same email (user already exists and is onboarded). A brand-new email that has never hit verify-code may land on `/onboarding` after the code — that is still authenticated; do not call it a login failure.

- **Render form.** Open `/login`. The heading `Sign in to Multica` is visible, textbox `Email` is visible, placeholder `you@example.com`, button `Continue` is disabled.
- **Send code.** Fill `Email` with the disposable address from this run (or `dev@localhost` only if you are proving the make-up human account and will not log that account out of the operator's browser). Choose `Continue`. Heading becomes `Check your email`.
- **Verify.** Focus the OTP textbox (hidden; `getByRole("textbox", { includeHidden: true })` or type into the focused OTP). Type the 6-digit code. Submit happens at the sixth digit. Do not click verify twice.
- **Land.** An onboarded user with a workspace arrives at `/{slug}/issues` with button `New Issue`. A brand-new user may arrive at `/onboarding` — that is still authenticated; do not file it as a login failure.
- **Anonymous redirect.** In a new empty context, open `/{known-slug}/issues`. URL becomes `/login` and `Sign in to Multica` is visible.
- **Log out.** From a signed-in issues page, choose the workspace switcher button (accessible name contains the workspace name) then menuitem `Log out`. URL is `/login` and `Sign in to Multica` is visible.
- **Proof.** Screenshot the email step and the post-verify destination. Record URL + heading. Artifacts: `artifacts/<run-id>/login/`.

## Gotchas

- Repeated `verify-code` with the same code locks it out (HTTP 400). Wait ~40s; do not hammer.
- Without `MULTICA_DEV_VERIFICATION_CODE`, the code is printed on the API stdout / log, not shown in the UI. Read the api log for this environment rather than guessing.
- Google sign-in (`Continue with Google`) needs a configured client id. Treat it as `verified-unreachable` unless that is configured; do not click it in a local proof.
- Logging out of `dev@localhost` in a shared browser signs the operator out. Never do that in their profile.
- `localStorage.multica_token` injection is setup for other features. It is not a proof of this feature.

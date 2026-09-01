# Local LAN self-host

Isolated Docker stack for this machine. It wraps the upstream compose files
and does not edit them.

- Project name: `multica-local`
- Web: http://192.168.10.118:3100  (also http://localhost:3100)
- API: http://192.168.10.118:8180
- Env file: repo-root `.env.selfhost.local` (gitignored)

## Login

There is no username/password.

This stack runs `APP_ENV=development` with a fixed verification code:

1. Open the Web URL
2. Enter any email
3. Code: `888888`

Anyone on the LAN who knows an email can log in with that code. Do not expose
these ports to the public internet.

## Commands

Run from the git tree you want inside the image (usually this checkout):

```bash
bash downstream/local-selfhost/selfhost-local.sh build    # rebuild from current source and replace
bash downstream/local-selfhost/selfhost-local.sh up       # recreate from current images/env, no rebuild
bash downstream/local-selfhost/selfhost-local.sh status
bash downstream/local-selfhost/selfhost-local.sh logs
bash downstream/local-selfhost/selfhost-local.sh stop     # keep volumes
```

Or:

```bash
make -C downstream/local-selfhost build
make -C downstream/local-selfhost up
```

`build` keeps the Postgres volume and only replaces backend/web images.
`up` is enough after changing bind address, CORS, or ports.

## First-time env

If `.env.selfhost.local` is missing:

```bash
cp downstream/local-selfhost/env.example .env.selfhost.local
# fill JWT_SECRET, POSTGRES_PASSWORD, MULTICA_VCS_SECRET_KEY
# set FRONTEND_ORIGIN / CORS to this machine's LAN IP
```

## How it avoids upstream conflicts

- Upstream compose still binds `127.0.0.1`
- `compose.bind.yml` uses Compose `!override` to republish on `0.0.0.0`
- Root `Makefile` is not patched

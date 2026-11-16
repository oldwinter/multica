import type { E2EConfig } from 'e2e';
import { web } from '@e2e-dev/web';

// The web target only needs the Next.js app. The API, database and daemon are
// not required for the landing-page smoke test, so the runner starts the web
// dev server alone. Point E2E_APP_URL at an already-running instance to reuse
// it instead (the command then reports APP_ALREADY_RUNNING).
const port = process.env.E2E_WEB_PORT ?? '3000';
const url = process.env.E2E_APP_URL ?? `http://localhost:${port}`;

export default {
  targets: [
    {
      name: 'web',
      engine: web(),
      app: {
        url,
        command: {
          executable: 'pnpm',
          args: ['dev'],
          cwd: 'apps/web',
          env: { FRONTEND_PORT: port },
          inheritEnv: ['PNPM_*', 'COREPACK_*', 'NODE_*', 'MISE_*'],
          reuseExisting: true,
          log: '.e2e/logs/web-dev.log',
          startupTimeout: 180_000,
        },
      },
    },
  ],
  output: '.e2e',
} satisfies E2EConfig;

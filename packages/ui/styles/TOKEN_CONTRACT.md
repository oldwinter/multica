# Appearance Token Contract

`packages/core/constants/semantic-token-schema.ts` is the platform-neutral source of truth for semantic roles and contrast requirements. Web/Desktop CSS and the Mobile palette validator both consume the versioned schema, so a role addition cannot land on only one platform. Contract version 1 resolves `tension`, `relay`, and `field` in both light and dark mode. `system` is intentionally absent because it resolves to one of those two modes.

Run the contract gate with:

```bash
corepack pnpm --filter @multica/ui check:tokens
```

The gate rejects missing tokens, invalid references, reference cycles, status-color collisions, critical contrast failures, raw product colors in shared UI primitives, and product behavior across application modules that branches on a skin name. It also verifies the shared forced-colors and reduced-motion guards.

## Changing The Contract

1. Add the role to `SEMANTIC_TOKEN_ROLES`, the CSS role mapping, and the Mobile role mapping. Define its CSS token in `:root`; override it only where a skin or dark mode needs a different meaning-preserving value.
2. Add a shared contrast requirement when the role carries text, focus, control recognition, status, chart, selection, destructive, code, or disabled-state meaning.
3. Add the corresponding `@theme inline` mapping only when product code needs a Tailwind utility.
4. Run the contract gate, Mobile theme contract test, and the existing Web contrast suite before changing consumers.
5. Increment the integer contract version when renaming, removing, or changing the meaning of a role. Keep the old and new names together for one coordinated platform migration only when installed clients require it; otherwise migrate all callers atomically.

Product colors belong in `tokens.css`. `raw-color-policy.mjs` distinguishes legitimate palette/brand sources from legacy product debt. Debt is capped by exact file, value, and count, so new raw-color uses fail even when a file already has recorded debt. Do not add an entire-file exception for a product surface.

Docs and the public landing surface also retain aggregate migration ceilings. `RAW_COLOR_DEBT_BUDGETS` rejects increases in those broad legacy surfaces. When a touched surface moves a raw value to a semantic token, lower its exact allowance or aggregate budget in the same change; neither form of debt may be spent again.

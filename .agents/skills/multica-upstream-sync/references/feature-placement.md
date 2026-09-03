# Place downstream features away from upstream hotspots

For an implementation request, fetch `upstream/main`. For a planning-only
request, use the cached ref and label the result as provisional. Record the
exact ref before inspecting the history of every shared path the feature may
touch:

```bash
git fetch upstream main
git log -1 --format='%H %cI' upstream/main
git log --oneline upstream/main -- <candidate-path>
```

Treat an upstream-owned hub or a path changed across recent upstream releases
as a hotspot. Touch a hotspot only for a narrow registration. Move the rest of
the behavior into an owned leaf or adapter.

Put new Rooms, Twin, Wiki, skin, or downstream operations behavior in the
owned leaves listed in the sync history. Give a new downstream domain its own
cohesive leaf and add that ownership to the map in the feature change.
Register one symbol at an existing shared point. If no suitable point exists,
add the narrowest extension point that avoids copying an upstream lifecycle
function.

For a shared menu, keep the action implementation in the owned leaf. Add one
registration or callback to the shared menu, and preserve its ordering,
grouping, visibility, and permission rules. Test both the registration and the
leaf action.

Make each shared list mechanically mergeable. Keep one entry per line and
group or sort entries without reformatting unrelated entries. Resolve source
files and regenerate `pnpm-lock.yaml`, sqlc output, and reserved-slug output.
When a package or toolchain version changes, update every derived consumer and
run `pnpm check:toolchain`. A manifest-only version bump is incomplete.

Completion criterion: the feature owns a leaf, each upstream-owned edit is one
named registration, and generated or derived files have a reproducible source.

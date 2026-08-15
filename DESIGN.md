# Multica Tension Map Design System

## 1. Atmosphere

Multica is an operating surface where humans, agents, issues, and runs stay in balance. The visual world is **Tension Map**: pale structural surfaces, dark working planes, taut signal lines, and one high-chroma accent that marks where attention is under load. It should feel engineered, calm, and legible under sustained daily use.

The system avoids generic AI gradients, terminal cosplay, decorative dashboards, nested cards, and ornamental network diagrams. Dependency geometry may appear as a restrained line, rail, or status trace, but content remains primary.

## 2. Palette And Named Skins

Every skin implements the same semantic token contract in light and dark modes. Skin and appearance are independent preferences. The default skin is `tension`; the default appearance is `system`.

| Skin | Light material | Dark material | Primary signal | Secondary signal |
| --- | --- | --- | --- | --- |
| Tension | concrete white | carbon black | tension red | safety amber |
| Relay | cool mineral gray | graphite | relay teal | signal coral |
| Field | mineral white | deep moss-charcoal | field green | survey amber |

Required semantic groups: canvas, shell, surface, raised surface, hover, selected, border, foreground, muted foreground, primary, destructive, success, warning, info, chart series, sidebar, focus ring, and code surface. Components consume only semantic tokens. No raw colors are introduced in product components.

## 3. Typography

Inter remains the UI face for Latin and the existing locale-specific system stacks remain authoritative for CJK. Geist Mono is reserved for code, identifiers, timestamps, measurements, and compact operational metadata. Source Serif 4 is reserved for editorial landing and onboarding statements.

All letter spacing is `0`. Hierarchy comes from size, weight, line height, contrast, and space. Dense product surfaces use restrained heading sizes; hero-scale type is limited to true landing or onboarding heroes.

## 4. Layout

The app shell owns viewport scrolling. Navigation, title bars, and toolbars remain stable while lists and detail panes own their local overflow. Operational pages prioritize scanning: compact rows, aligned control columns, stable icon targets, and predictable split panes.

Page sections are unframed. Cards are limited to repeated records, settings groups, dialogs, and genuinely bounded tools. Cards never nest. Product card radius is at most `8px`; mobile follows native grouped-section conventions with the same visual contract.

## 5. Components And States

- `ThemeProvider`: owns the independent skin preference and the existing light/dark/system preference, persistence, document attributes, cross-tab sync, and reduced-motion-aware transitions.
- `SkinSwatch`: a selectable three-tone material preview with name, description, selected state, keyboard focus, and full-row hit target.
- `AppearanceSegment`: System, Light, and Dark modes presented as a stable segmented control.
- `SignalRail`: a one-pixel semantic line used sparingly for active relationships, progress, or selection; never as a decorative thick card edge.
- `ThemePreview`: the Settings appearance section acts as the primitive showcase, exercising canvas, surface, text, border, focus, status, and primary action tokens before the rest of the product consumes them.

Every interactive primitive defines default, hover/pressed, focus-visible, selected, disabled, loading, error, and empty behavior where applicable.

## 6. Motion

The authored moment is theme switching. Explicit skin or appearance changes use the View Transition API to reveal the new theme from the control's position with a viewport clip-path over approximately 400ms and an ease-out curve. Unsupported browsers and `prefers-reduced-motion` switch immediately. System preference changes do not animate.

All other motion remains functional: disclosure, loading, drag, and pane transitions only. No decorative staggered entrances or looping ambient effects.

## 7. Depth

Depth is carried primarily by borders and tonal separation. Raised overlays may use a small offset with a soft neutral shadow; permanent page structure does not float. Popovers, dialogs, and menus are the only routine elevated layers. Colored halos and hard offset shadows are not part of this world.

## 8. Accessibility And Debt

Target WCAG 2.2 AA. Body and placeholder text require 4.5:1 contrast; large text and essential graphical objects require 3:1. Focus rings must remain visible in every skin/mode combination. Controls keep at least a 44px touch target on mobile and an ergonomic icon target on desktop. All theme controls expose names and selected state to assistive technology.

Reduced motion, keyboard navigation, system appearance changes, CJK wrapping, Windows high contrast, and zoom at 200% are required verification cases. Existing landing and docs raw-color islands are design debt and must be converted to semantic variables as their surfaces are touched by this redesign.

# Mini-Inference UI Design System

## Product posture

Mini-Inference is a private-network operations console for one maintainer. The interface must answer four questions in order:

1. Is the service safe to use now?
2. Is the model loaded and accepting requests?
3. Is a request active or waiting?
4. Are resources, alerts, and backups healthy?

The visual language is calm, minimal, technical, and trustworthy. It must not resemble a marketing dashboard or a chat product.

## Foundation

- Component system: IBM Carbon only.
- Theme: one light theme.
- Canvas: cool gray `#f3f6f5`.
- Surface: white `#ffffff` and muted surface `#f7f9f8`.
- Primary text: deep ink `#15201d`; secondary text `#66736f`.
- Accent: teal `#087f68`, used for navigation, focus, links, and positive live-state emphasis.
- Semantic colors remain reserved for success, warning, failure, transition, and unknown states.
- Typography: IBM Plex Sans with Chinese system fallbacks; IBM Plex Mono for identifiers and measured values.
- Radius rule: panels 12px, nested blocks 8–10px, buttons 8px, status tags pill-shaped. No unrelated radius values.
- Motion: 120–180ms state and feedback transitions only. No decorative looping motion.

## Layout rules

- Header height is 64px. Desktop navigation is 240px wide.
- Main content is capped at 1440px and follows a 12-column mental grid.
- Page headers state the task, not the implementation.
- Overview order is status strip, model/current execution, resources, queue/attention.
- Cards are used only for independently scannable state. Tables and long sections use a single contained surface.
- At 768–1279px, multi-column work areas collapse to one column and resources become 2×2.
- Below 768px, all state groups are single-column; only data tables may scroll horizontally.

## Component rules

- Status tags always contain text and an icon; color is never the sole signal.
- Primary action uses the accent color. Destructive actions use a restrained outlined treatment until confirmation.
- Empty states preserve the expected region and explain the absence without pretending it is healthy.
- Data cards show label, primary value, source, and sampling context in that order.
- Tables keep native semantics, visible captions, fixed backend ordering, and local horizontal scrolling.
- Focus uses a visible 2px accent outline. Interactive targets are at least 32px high; primary actions are at least 44px on mobile.

## Standard page design and development flow

Every new or revised page follows this chain:

1. **Intent brief** — define the user, the page’s single main job, the top three questions it must answer, and explicit non-goals.
2. **State inventory** — list authoritative data, loading, empty, partial, stale, error, permission, transition, and destructive-action states before drawing layout.
3. **Information hierarchy** — order facts by operational importance; assign one `h1`, meaningful `h2` regions, primary action, and escape/recovery paths.
4. **Wireframe** — produce desktop and narrow-screen structures using real labels and representative data. No color polish yet.
5. **Token mapping** — map spacing, type, color, border, radius, and motion to this file and Carbon tokens. New one-off values require a documented reason.
6. **Component contract** — define props, backend ownership, interaction states, keyboard behavior, responsive fallback, and failure behavior.
7. **Implementation** — consume generated API types, preserve DOM/visual order, and reuse existing patterns before creating new abstractions.
8. **Browser acceptance** — verify at 320×720, 768×900, 1440×900; check keyboard flow, focus return, reduced motion, overflow, empty/error states, and real backend data.
9. **Review gate** — compare implementation against the intent brief and state inventory. Reject decorative controls, fake data, duplicated actions, or client-invented status.
10. **Documentation update** — durable visual rules return here; wire contracts and state transitions belong in `docs/technical/frontend.md`; one-off findings remain in the change report.

## Definition of done

A page is done when its main question is answerable within one viewport at desktop, every backend state has an explicit rendering, narrow screens avoid page-level horizontal scroll, keyboard operation is complete, and the real Compose-served page passes browser review. A screenshot alone is not acceptance.

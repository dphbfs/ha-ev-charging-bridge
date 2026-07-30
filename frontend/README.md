# V2 Frontend Mockups

This folder contains static HTML/CSS mockups for the V2 Home Assistant EV charging sessions UI.

## Files

- `index.html` renders the Charging Sessions overview.
- `session.html` renders the Session Details view.
- `styles.css` contains shared responsive light/dark styling.
- `app.js` renders representative sample history rows/cards and static chart SVGs.

## Run Locally

Open the files directly in a browser, or serve the folder with any static file server:

```sh
python3 -m http.server 8080 --directory frontend
```

Then open:

- `http://localhost:8080/index.html`
- `http://localhost:8080/session.html`

Use `?theme=light` to start either page in light mode, for example:

- `http://localhost:8080/index.html?theme=light`
- `http://localhost:8080/session.html?theme=light`

Each page also includes a theme toggle.

## Validation Checklist

Validate both pages in dark and light mode at these viewport widths:

- 320px
- 375px
- 390px
- 430px
- 768px
- 1024px
- 1366px
- 1536px

Check that:

- Overview and details pages match the issue #9 mockups in structure, spacing, color, and hierarchy.
- Desktop uses the Home Assistant-style sidebar shell.
- Mobile uses a compact top bar and stacked content.
- Session history is a table on desktop and stacked cards on mobile.
- Chart labels remain readable.
- Important text is readable or intentionally constrained.
- Controls, badges, cards, table rows, and timeline entries do not overlap.
- Switching themes does not cause jumpy layout changes.

## Sample Data Coverage

The static UI includes representative data for:

- Active charging session.
- Completed sessions.
- Stopped session.
- Interrupted session.
- Unknown-status session.
- Long charger and connector names.
- Large meter reading and energy values on the details page.

## Intentional Deviations

- Charts are static SVG renderings, not interactive chart components.
- Filter controls are visual only in this task.
- Raw data is a small static example.
- Home Assistant icons are approximated with text glyphs until the Lit/TypeScript implementation introduces a proper icon strategy.

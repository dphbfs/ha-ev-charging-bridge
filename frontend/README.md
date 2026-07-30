# V2 Frontend

This folder contains the V2 Home Assistant EV charging sessions UI. The current implementation uses Lit + TypeScript with mock data so the UI behaves like the future data-backed app before backend API integration is available.

## Files

- `index.html` hosts the Charging Sessions overview app.
- `session.html` hosts the Session Details route directly.
- `components.html` hosts the component validation gallery.
- `styles.css` contains shared responsive light/dark styling.
- `src/main.ts` defines the Lit components and interactive app state.
- `src/mock-data.ts` contains mock active session, history, meter sample, and timeline data.
- `src/types.ts` contains the frontend session data types.

## Install

From `frontend/`:

```sh
npm install
```

## Run Locally

From `frontend/`:

```sh
npm run dev
```

Then open the Vite dev server URL shown in the terminal.

Useful routes:

- `/index.html` for the overview screen.
- `/session.html` for the detail screen.
- `/components.html` for the component validation surface.

Use `?theme=light` to start a page in light mode, for example:

- `/index.html?theme=light`
- `/session.html?theme=light`

Each page also includes a theme toggle.

## Home Assistant Dev Mode

Use `HOME_ASSISTANT_DEV.md` for the recommended Home Assistant dev loading workflow. The preferred path is a temporary Supervisor app/add-on with Ingress enabled, which runs the Vite dev server on port `8099` and opens the UI through Home Assistant's `OPEN WEB UI` flow.

## Build

From `frontend/`:

```sh
npm run build
```

The build output is written by Vite to `frontend/dist/`.

## Interactions

The mock-data app supports:

- Overview search by session, charger, connector, or EVSE fields.
- Status filtering.
- Newest/oldest sorting.
- Refresh timestamp updates.
- Desktop pagination and mobile load-more behavior.
- Overview-to-detail navigation.
- Detail back navigation.
- Copy session ID affordance.
- Power and energy chart visibility toggles.
- Reset chart state.
- Raw data expansion.
- Timeline meter sample expansion affordance.

Mock data remains local to `src/mock-data.ts` and is shaped to be replaced by API DTOs in a later task.

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
- Search, filters, sort, pagination/load more, detail navigation, chart toggles, copy, and accordions respond to user input.

Also validate `components.html` in light and dark mode with narrow and wide viewports. The gallery includes status badges, long text, large numeric values, chart states, and no-data chart state.

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

- Charts are lightweight SVG Lit components rather than a production charting dependency.
- Date range and charger controls are represented visually; status/search/sort are interactive.
- Raw data uses the selected mock session.
- Home Assistant icons are approximated with text glyphs until the Lit/TypeScript implementation introduces a proper icon strategy.

## Future API Integration

The next frontend data task should add a small API client module and replace `src/mock-data.ts` as the default data source. Keep the component props/view models close to the current `ChargingSession`, `MeterSample`, and `TimelineEvent` types so backend DTO mapping stays isolated.

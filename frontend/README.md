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

The app loads session data from the backend API by default and falls back to mock data when the API is unavailable. It supports:

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

Mock data remains local to `src/mock-data.ts` for the component gallery and offline fallback.

## API Configuration

The frontend resolves the backend API base URL in this order:

1. `?apiBaseUrl=http://host:port` query parameter.
2. `window.EV_CHARGING_API_BASE_URL` runtime global.
3. `VITE_API_BASE_URL` build/dev environment variable.
4. Empty string, which uses same-origin relative API paths.

Examples:

```sh
VITE_API_BASE_URL=http://127.0.0.1:8080 npm run dev
```

Or:

```text
http://localhost:5173/index.html?apiBaseUrl=http://127.0.0.1:8080
```

For Home Assistant ingress, prefer same-origin relative paths unless the backend is deliberately exposed on a separate development origin.

## Run Against The Go Backend

Start the Go bridge with the HTTP API enabled:

```sh
API_ADDR=127.0.0.1:8080 go run .
```

Then start the frontend:

```sh
cd frontend
VITE_API_BASE_URL=http://127.0.0.1:8080 npm run dev
```

Required backend endpoints:

- `GET /api/v1/sessions`
- `GET /api/v1/sessions/active`
- `GET /api/v1/sessions/{session_id}`

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

API mapping lives in `src/api-client.ts`. Keep backend DTO changes isolated there so Lit components can continue using the `ChargingSession`, `MeterSample`, and `TimelineEvent` view models.

## Troubleshooting API Loading

- If the UI shows an API error banner, confirm the Go backend is running and `API_ADDR` matches the frontend API base URL.
- If browser dev tools show CORS failures, use same-origin proxying/ingress or configure backend CORS in a future task.
- If Home Assistant ingress returns wrong paths, use relative same-origin API URLs and avoid hard-coded private hostnames.
- If there are no stored sessions yet, the API can return an empty history; the component gallery still shows mock states.

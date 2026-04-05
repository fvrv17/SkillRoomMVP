# Frontend Agent

## Design

The frontend is a Next.js app with:

- a launch/auth scene
- guest preview mode
- challenge workspace
- room view
- rankings
- recruiter candidate cards and filters

The visual system uses orange, black, and cream, with the room mapped to real skill data.

## Implementation

Core files:

- `frontend/app/page.js`
- `frontend/app/landing-client.js`
- `frontend/app/workspace/page.js`
- `frontend/app/workspace/workspace-client.js`
- `frontend/app/backend/[...path]/route.js`
- `frontend/app/globals.css`
- `frontend/lib/client.js`
- `frontend/lib/preview-data.js`

Current room mapping in the UI:

- monitor / react
- desk / javascript
- chair / architecture
- plant / consistency
- shelf / solved volume
- trophy case / percentile

## Tests

- `internal/backend/frontend_test.go`
- `node --check frontend/app/workspace/workspace-client.js`

## Tradeoffs

- preview mode uses seeded data, but logged-in flows use the real backend
- chat is still backend-driven request/response rather than a dedicated realtime phase 2 implementation

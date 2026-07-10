# scheduling

A Doodle-style scheduling app implemented as a monorepo with a Go API and SvelteKit web app.

## Features

- User signup, login, logout, and cookie-backed sessions
- Logged-in users can create scheduling surveys
- Interactive calendar/time-slot picker for candidate availability
- Shareable public survey links
- Respondents can select every offered slot that works for them
- Creators can view aggregate availability results

## Monorepo layout

```text
apps/api  Go REST API, SQLite persistence, auth/session logic
apps/web  SvelteKit frontend
```

## Requirements

- Go 1.26+
- Node 22+ / npm

## Development

```bash
# API, defaults to :8080 and ./scheduling.db
make dev-api

# Web, proxies API calls to VITE_API_BASE or http://localhost:8080
make dev-web
```

The web app expects `VITE_API_BASE` to point at the API origin. In local development it defaults to `http://localhost:8080`.

## Verification

```bash
make test
make build
```

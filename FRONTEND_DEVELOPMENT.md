# Frontend development guide

This backend is ready for a browser-based frontend. Use this guide to wire your app to the APIs.

## 1. Run backend with CORS

The server allows cross-origin requests from configured origins so your frontend (e.g. on another port) can call the API.

- **Default:** `http://localhost:3000` is allowed (no env var needed).
- **Custom:** set `CORS_ORIGINS` to a comma-separated list:
  ```bash
  CORS_ORIGINS=http://localhost:5173,https://myapp.example.com
  ```
- **Disable CORS:** set `CORS_ORIGINS=` (empty).

Start the backend as usual (e.g. `go run ./cmd/server` or your Makefile). Frontend and backend can run on different ports.

## 2. API base URL

- **Local:** `http://localhost:8080` (or whatever `PORT` is in your `.env`).
- In the frontend, use an env var so you can switch for staging/production, e.g.:
  - Vite: `import.meta.env.VITE_API_URL` or `VITE_API_URL=http://localhost:8080`
  - Create React App: `REACT_APP_API_URL`
  - Next.js: `NEXT_PUBLIC_API_URL`

All API requests should go to `{baseUrl}/routes/...` or `{baseUrl}/auth/...`.

## 3. Authentication (OAuth + cookie)

Auth is **cookie-based**: after login, the server sets a **session cookie** (JWT). The browser sends it automatically with same-origin or CORS requests when credentials are included.

### Login flow

1. **Redirect the user to the backend login URL:**
   ```
   GET {baseUrl}/auth/google/login
   ```
   (or `/auth/github/login` if you configure GitHub.) The user is redirected to Google, then back to your backend callback, which sets the session cookie and redirects to `OAUTH_SUCCESS_URL` (e.g. your frontend home).

2. **Configure OAuth success redirect:**  
   Set `OAUTH_SUCCESS_URL` to your frontend URL so after login the user lands on your app, e.g.:
   ```
   OAUTH_SUCCESS_URL=http://localhost:3000/
   ```

3. **API calls from the frontend:**  
   Use **credentials: 'include'** (fetch) or **withCredentials: true** (axios) so the session cookie is sent:
   ```javascript
   fetch(`${API_URL}/routes/my`, { credentials: 'include' })
   ```

### Logout

- **GET** `{baseUrl}/auth/logout` (with credentials). The server clears the session cookie.

### Checking if the user is logged in

- Call a protected endpoint (e.g. **GET** `/routes/my`) with credentials. If you get **200** you’re logged in; **401** means not authenticated.

## 4. Routes API summary

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/routes/{id}` | No | Get one route by ID |
| POST | `/routes/search` | No | Search routes (body: start/end coords, stops) |
| POST | `/routes` | Yes | Create a route |
| GET | `/routes/my` | Yes | Routes I created (`?filter=active` or `past`) |
| GET | `/routes/participated` | Yes | Routes I participated in (`?filter=active` or `past`) |

Request/response shapes: see the backend handlers and types (e.g. `internal/handlers/routes.go`, `internal/route/route.go`). Use the same JSON fields in your frontend types.

## 5. Suggested frontend stack

- **React** (Vite or CRA) or **Vue** or **Next.js** – any stack that can call REST APIs and send cookies.
- **Fetch** or **axios** with `credentials: 'include'` / `withCredentials: true`.
- Store **API base URL** in env (e.g. `VITE_API_URL`) and use it for all `fetch(API_URL + '/routes/...')` calls.

## 6. Example: fetch wrapper

```javascript
const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080';

export async function api(path, { method = 'GET', body, ...rest } = {}) {
  const opts = {
    method,
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...rest.headers },
    ...rest,
  };
  if (body != null) opts.body = JSON.stringify(body);
  const res = await fetch(`${API_URL}${path}`, opts);
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || res.statusText);
  }
  return res.json();
}

// Usage:
// const routes = await api('/routes/my');
// const created = await api('/routes', { method: 'POST', body: { description: '...', start_lat: 54.9, ... } });
```

## 7. Project layout (optional)

You can keep the frontend in a separate repo or in a subfolder of this repo, e.g.:

- `pss-backend/` (this repo) – Go API
- `pss-frontend/` – separate repo or `pss-backend/frontend/` – React/Vue/Next app

If you use a monorepo, run backend and frontend dev servers side by side (e.g. backend on 8080, Vite on 3000), with `CORS_ORIGINS` including the frontend origin.

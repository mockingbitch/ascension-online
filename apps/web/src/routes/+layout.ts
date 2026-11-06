// SPA mode: the access token lives in memory, and /api is served by the Go API (via Vite proxy in dev,
// Nginx in prod), so all data loading happens in the browser.
export const ssr = false;

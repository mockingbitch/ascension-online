// Offline fallback (spec §4.11): prerendered at build, served by the service worker only when a page navigation
// fails because the network is down. Static HTML + CSS (tokens from app.css), no JS, no data, no auth state.
export const prerender = true;
export const ssr = true;
export const csr = false;

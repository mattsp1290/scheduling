// Default to same-origin ('') so the production bundle issues relative /api
// requests (served by Traefik on the same host). In dev, Vite's `/api` proxy
// (vite.config.ts → VITE_API_PROXY) forwards to the local API. Set
// VITE_API_BASE to an absolute origin only for cross-origin setups.
const API_BASE = import.meta.env.VITE_API_BASE ?? '';

export async function apiFetch(path, options = {}) {
	const response = await fetch(`${API_BASE}${path}`, {
		...options,
		credentials: 'include',
		headers: {
			'Content-Type': 'application/json',
			...(options.headers || {})
		}
	});
	const text = await response.text();
	let body = null;
	if (text) {
		try {
			body = JSON.parse(text);
		} catch {
			body = { error: `Non-JSON response (${response.status}): ${text.slice(0, 120)}` };
		}
	}
	if (!response.ok) {
		throw new Error(body?.error || `Request failed with ${response.status}`);
	}
	return body;
}

export function publicLink(token) {
	return `${window.location.origin}/s/${token}`;
}

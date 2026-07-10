const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080';

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
	const body = text ? JSON.parse(text) : null;
	if (!response.ok) {
		throw new Error(body?.error || `Request failed with ${response.status}`);
	}
	return body;
}

export function publicLink(token) {
	return `${window.location.origin}/s/${token}`;
}

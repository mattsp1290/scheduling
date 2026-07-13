import adapter from '@sveltejs/adapter-static';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	kit: {
		// SPA mode: the app is fully client-side (no server routes/hooks/actions).
		// `fallback` makes every unmatched path (incl. dynamic routes like
		// /s/[token] and /surveys/[token]/results) serve index.html so the
		// client router can take over. Served as static files by the
		// scheduling-web httpd container.
		adapter: adapter({ fallback: 'index.html' }),
		// CSP is owned HERE (emitted as a <meta http-equiv> in the fallback
		// index.html) rather than by a Traefik response header. hash mode makes
		// SvelteKit add a sha256 hash for its inline bootstrap script, so
		// `script-src 'self'` does not block it. Two independent CSPs (meta +
		// header) are each enforced, so a Traefik `script-src 'self'` header
		// WITHOUT the hash would break the inline script — the scheduling
		// Traefik middleware therefore sets frameDeny/HSTS/etc. but NO CSP
		// header. frame-ancestors is not expressible via meta, so clickjacking
		// is covered by Traefik's frameDeny (X-Frame-Options: DENY) instead.
		// Deliberately NO default-src or style-src here. In hash mode SvelteKit
		// hashes every inline <script> AND <style> it emits; once a style hash is
		// present the browser IGNORES 'unsafe-inline', which would block Svelte 5's
		// runtime inline style= attributes (transitions/animations). Omitting
		// style-src (and the default-src that would otherwise catch styles) leaves
		// styles unrestricted while still locking scripts to self + the bootstrap
		// hash — the directive that actually matters for XSS. connect-src is
		// same-origin (API is same-host); object-src/base-uri close plugin and
		// base-tag vectors.
		csp: {
			mode: 'hash',
			directives: {
				'script-src': ['self'],
				'connect-src': ['self'],
				'img-src': ['self', 'data:'],
				'object-src': ['none'],
				'base-uri': ['self']
			}
		}
	}
};

export default config;

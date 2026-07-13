// SPA mode for adapter-static: disable SSR and prerendering so the app ships
// as a single client-rendered bundle with an index.html fallback. Required
// because routes like /s/[token] and /surveys/[token]/results are dynamic and
// depend on the separate Go API at runtime.
export const ssr = false;
export const prerender = false;
export const trailingSlash = 'ignore';

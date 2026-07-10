import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [sveltekit()],
	server: {
		allowedHosts: ['web'],
		proxy: {
			'/api': process.env.VITE_API_PROXY || 'http://localhost:8080'
		}
	}
});

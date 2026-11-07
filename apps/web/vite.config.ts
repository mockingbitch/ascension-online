import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		// Go API (services/game) in dev. In prod Nginx routes /api → api container.
		proxy: {
			'/api': process.env.API_ORIGIN ?? 'http://localhost:2080',
			'/ws': { target: (process.env.API_ORIGIN ?? 'http://localhost:2080').replace(/^http/, 'ws'), ws: true }
		}
	},
	test: { include: ['src/**/*.test.ts'] }
});

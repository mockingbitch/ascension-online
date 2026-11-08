// Design tokens → canvas colours. A hidden probe element resolves `var(--color-…)` through the cascade; the
// ParticleSystem caches each result, so this runs once per colour. Returns the resolver and its cleanup.
export function tokenResolver(): { resolve: (color: string) => string; dispose: () => void } {
	const probe = document.createElement('span');
	probe.style.display = 'none';
	document.body.append(probe);
	return {
		resolve: (color) => {
			probe.style.color = color;
			return getComputedStyle(probe).color;
		},
		dispose: () => probe.remove()
	};
}

<script lang="ts">
	import type { Aura } from '$lib/game/types';

	interface Props {
		name: string;
		aura: Aura;
		size?: 'sm' | 'md' | 'lg';
	}
	let { name, aura, size = 'md' }: Props = $props();

	const sizes = { sm: 'h-7 px-3 text-base', md: 'h-9 px-4 text-lg', lg: 'h-12 px-6 text-3xl' };
</script>

<!-- Cut-corner plaque: outer layer = aura-tinted rim, inner layer = ink face; halo sits outside the clip. -->
<span class="relative inline-flex" style:--aura="var(--color-aura-{aura})">
	{#if aura !== 'none'}
		<span aria-hidden="true" class="halo absolute -inset-1 animate-qi-breathe blur-md"></span>
	{/if}
	<span class="plaque relative inline-flex p-px">
		<span class="plaque face inline-flex items-center font-display leading-none font-bold whitespace-nowrap {sizes[size]}">
			{name}
		</span>
	</span>
</span>

<style>
	.plaque {
		--cut: 7px;
		clip-path: polygon(var(--cut) 0, calc(100% - var(--cut)) 0, 100% var(--cut), 100% calc(100% - var(--cut)), calc(100% - var(--cut)) 100%, var(--cut) 100%, 0 calc(100% - var(--cut)), 0 var(--cut));
		background: linear-gradient(180deg, color-mix(in oklab, var(--aura) 85%, transparent), color-mix(in oklab, var(--aura) 35%, transparent));
	}
	.face {
		color: var(--aura);
		background:
			radial-gradient(ellipse at 50% 0%, color-mix(in oklab, var(--aura) 22%, transparent), transparent 70%),
			linear-gradient(180deg, var(--color-ink-800), var(--color-ink-950));
		text-shadow: 0 0 12px color-mix(in oklab, var(--aura) 45%, transparent);
	}
	.halo {
		background: color-mix(in oklab, var(--aura) 22%, transparent);
	}
</style>

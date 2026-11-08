<!-- Help ("?"): the rules of one screen, as short points. -->
<script lang="ts">
	import Icon from '$lib/components/Icon.svelte';
	import { help } from '$lib/game/help';
	import type { WindowProps } from './registry';

	let { win }: WindowProps = $props();
	const entry = $derived(win.kind === 'help' ? help[win.topic] : null);
</script>

{#if entry}
	<ul class="stagger space-y-3" data-help-topic={win.kind === 'help' ? win.topic : ''}>
		{#each entry.points as p, i (i)}
			<li class="flex gap-3 text-sm leading-relaxed text-mist-100">
				<span class="mt-0.5 text-gold-500"><Icon name={i === 0 ? entry.icon : 'spark'} size={16} /></span>
				<span>{p}</span>
			</li>
		{/each}
	</ul>
{/if}

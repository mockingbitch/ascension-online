<!-- Help ("?"): the rules of one screen, as short points — or Vân Khê explains them (summon, not stored). -->
<script lang="ts">
	import Icon from '$lib/components/Icon.svelte';
	import { help } from '$lib/game/help';
	import { tutorial } from '$lib/game/tutorial.svelte';
	import { closeWindow } from '$lib/game/windows';
	import type { WindowProps } from './registry';

	let { win }: WindowProps = $props();
	const entry = $derived(win.kind === 'help' ? help[win.topic] : null);
	function ask() {
		if (win.kind !== 'help') return;
		const topic = win.topic;
		closeWindow();
		tutorial.summon(topic);
		void tutorial.loadScript();
	}
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
	<button type="button" class="btn btn-ghost mt-5 h-11 w-full text-sm" onclick={ask} data-ask-guide>
		<Icon name="person" size={16} /> Nhờ Vân Khê giải thích
	</button>
{/if}

<!-- "?" — opens the help window for a screen's rules (spec §3.4: explanations live behind it, not on the screen). -->
<script lang="ts">
	import { openWindow } from '$lib/game/windows';
	import type { HelpTopic } from '$lib/game/windowStack';
	import Icon from './Icon.svelte';

	let { topic, class: cls = '' }: { topic: HelpTopic; class?: string } = $props();
</script>

<button type="button" class="help press {cls}" onclick={() => openWindow({ kind: 'help', topic })} data-help={topic}>
	<Icon name="help" size={20} label="Luật chơi" />
</button>

<style>
	.help {
		display: inline-grid;
		width: var(--tap);
		height: var(--tap);
		flex-shrink: 0;
		place-items: center;
		border-radius: 9999px;
		color: var(--color-gold-300);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 40%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 70%, transparent);
		pointer-events: auto;
	}
	.help:hover {
		background: color-mix(in oklab, var(--color-gold-500) 14%, var(--color-ink-950));
	}
</style>

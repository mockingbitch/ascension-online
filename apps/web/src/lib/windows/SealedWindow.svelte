<!-- A place that is not open yet (/play/<screen> without a screen): an in-world "sealed" state, not a 404. -->
<script lang="ts">
	import Icon from '$lib/components/Icon.svelte';
	import { screens } from '$lib/game/nav';
	import { openWindow } from '$lib/game/windows';
	import type { WindowProps } from './registry';

	let { win }: WindowProps = $props();
	const s = $derived(win.kind === 'sealed' ? Object.values(screens).find((x) => x.href === `/play/${win.screen}`) : undefined);
</script>

<div class="flex flex-col items-center py-6 text-center">
	<div class="seal grid size-24 place-items-center rounded-full text-gold-500">
		<Icon name={s?.icon ?? 'lock'} size={40} />
	</div>
	<p class="mt-5 font-display text-2xl font-bold text-gold-300">Cấm chế chưa giải</p>
	<p class="mt-1 max-w-sm text-sm text-mist-300">{s ? `${s.label} sẽ khai mở khi thiên đạo vận chuyển tới.` : 'Nơi này không tồn tại trong Hồng Hoang.'}</p>
	<button type="button" class="btn btn-ghost mt-5" onclick={() => openWindow({ kind: 'timeline' })}><Icon name="clock" size={16} /> Xem Thiên Đạo Sử</button>
</div>

<style>
	.seal {
		background: radial-gradient(circle, color-mix(in oklab, var(--color-gold-500) 14%, var(--color-ink-800)), var(--color-ink-950));
		box-shadow:
			0 0 0 1px var(--color-gold-500),
			0 0 0 7px var(--color-ink-950),
			0 0 0 8px color-mix(in oklab, var(--color-gold-500) 40%, transparent);
	}
</style>

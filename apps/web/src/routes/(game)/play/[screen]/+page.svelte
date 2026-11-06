<!-- Any /play/<screen> that is not built yet: an in-world "sealed" state instead of a 404 or a dead link. -->
<script lang="ts">
	import { page } from '$app/state';
	import Icon from '$lib/components/Icon.svelte';
	import ScreenFrame from '$lib/components/ScreenFrame.svelte';
	import { screens } from '$lib/game/nav';

	const s = $derived(Object.values(screens).find((x) => x.href === page.url.pathname));
</script>

<ScreenFrame title={s?.label ?? 'Chốn chưa khai mở'} icon={s?.icon ?? 'lock'}>
	<div class="flex h-full flex-col items-center justify-center py-10 text-center">
		<div class="seal grid size-28 place-items-center rounded-full text-gold-500">
			<Icon name={s?.icon ?? 'lock'} size={44} />
		</div>
		<h2 class="mt-6 font-display text-3xl font-bold text-gold-300">Cấm chế chưa giải</h2>
		<p class="mt-2 max-w-md text-mist-300">
			{s ? `${s.label} sẽ khai mở khi thiên đạo vận chuyển tới.` : 'Nơi này không tồn tại trong Thiên Nguyên Giới.'}
			Theo dõi Thiên Đạo Sử để biết khi nào cấm chế được phá.
		</p>
		<a href="/play" class="btn btn-ghost mt-6"><Icon name="map" size={16} /> Về bản đồ thế giới</a>
	</div>
</ScreenFrame>

<style>
	.seal {
		background: radial-gradient(circle, color-mix(in oklab, var(--color-gold-500) 14%, var(--color-ink-800)), var(--color-ink-950));
		box-shadow:
			0 0 0 1px var(--color-gold-500),
			0 0 0 8px var(--color-ink-950),
			0 0 0 9px color-mix(in oklab, var(--color-gold-500) 40%, transparent),
			0 0 40px -6px color-mix(in oklab, var(--color-gold-300) 40%, transparent);
	}
</style>

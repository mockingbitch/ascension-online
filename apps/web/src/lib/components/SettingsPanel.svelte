<!--
	Cài đặt: effects quality, sound (mute + a volume per channel, persisted), fullscreen, renderer flag. Client
	presentation only (lib/engine/settings.svelte.ts). Unmuting is a click, so it also unlocks audio right away.
-->
<script lang="ts">
	import { audio, type AudioChannel } from '$lib/engine/audio/AudioManager';
	import { fullscreen } from '$lib/engine/fullscreen.svelte';
	import type { Quality } from '$lib/engine/fx/particles';
	import type { Fallback3dReason } from '$lib/engine/render/select';
	import type { RendererKind } from '$lib/engine/render/types';
	import { settings } from '$lib/engine/settings.svelte';
	import Icon from './Icon.svelte';

	const fxOptions: { value: Quality; label: string; hint: string }[] = [
		{ value: 'full', label: 'Đầy đủ', hint: 'Mọi hiệu ứng, hạt linh khí, chuyển cảnh' },
		{ value: 'lite', label: 'Nhẹ', hint: 'Ít hạt hơn, hợp máy yếu' },
		{ value: 'off', label: 'Tắt', hint: 'Không hoạt ảnh trang trí' }
	];
	const rendererOptions: { value: RendererKind; label: string; hint: string }[] = [
		{ value: '2d', label: '2D (mặc định)', hint: 'Tranh vẽ 2D cho mọi cảnh' },
		{ value: '3d', label: '3D thử nghiệm (Tử Tiêu Cung)', hint: 'Chỉ Tử Tiêu Cung, cần WebGL2; máy yếu tự về 2D' }
	];
	const fallbackLabel: Record<Fallback3dReason, string> = {
		'no-webgl2': 'thiết bị không có WebGL2',
		'context-lost': 'mất ngữ cảnh đồ họa',
		slow: 'khung hình dưới 40 fps',
		error: 'lỗi khi tải 3D'
	};
	const channels: { id: AudioChannel; label: string }[] = [
		{ id: 'bgm', label: 'Nhạc nền' },
		{ id: 'ambience', label: 'Âm cảnh' },
		{ id: 'sfx', label: 'Hiệu ứng' },
		{ id: 'ui', label: 'Giao diện' }
	];

	function setMuted(muted: boolean) {
		settings.setMuted(muted);
		audio().configure(settings.audio);
		if (!muted) void audio().unlock(); // inside the click: the browser lets the sound start
	}
</script>

<div class="space-y-6 text-sm">
	<fieldset>
		<legend class="panel-title text-lg">Hiệu ứng</legend>
		<div class="mt-3 grid grid-cols-3 gap-2" role="radiogroup" aria-label="Mức hiệu ứng">
			{#each fxOptions as o (o.value)}
				<button
					type="button"
					role="radio"
					aria-checked={settings.fx === o.value}
					class="opt"
					class:is-on={settings.fx === o.value}
					title={o.hint}
					onclick={() => settings.setFx(o.value)}
				>
					{o.label}
				</button>
			{/each}
		</div>
		<p class="mt-2 text-xs text-mist-500">
			{#if settings.fxChoice === null}
				Đang theo thiết bị{settings.reducedMotion ? ' (giảm chuyển động → Tắt)' : ''}.
			{:else}
				<button type="button" class="text-gold-300 hover:underline" onclick={() => settings.setFx(null)}>Theo cài đặt thiết bị</button>
			{/if}
		</p>
	</fieldset>

	<fieldset>
		<legend class="panel-title text-lg">Âm thanh</legend>
		<label class="row mt-3">
			<Icon name={settings.audio.muted ? 'mute' : 'sound'} size={18} class="text-mist-300" />
			<span>Tắt tiếng</span>
			<input
				type="checkbox"
				class="ml-auto size-5 accent-jade-500"
				checked={settings.audio.muted}
				onchange={(e) => setMuted(e.currentTarget.checked)}
				data-setting-mute
			/>
		</label>
		{#each channels as c (c.id)}
			{@const v = Math.round(settings.audio.volumes[c.id] * 100)}
			<label class="row">
				<span class="w-24 shrink-0">{c.label}</span>
				<input
					type="range"
					min="0"
					max="100"
					step="5"
					value={v}
					disabled={settings.audio.muted}
					class="min-w-0 flex-1 accent-jade-500"
					aria-valuetext="{v}%"
					data-setting-volume={c.id}
					oninput={(e) => settings.setVolume(c.id, Number(e.currentTarget.value) / 100)}
				/>
				<span class="w-10 text-right tabular-nums text-mist-300">{v}%</span>
			</label>
		{/each}
		<p class="mt-2 text-xs text-mist-500">Âm thanh chỉ phát sau lần chạm đầu tiên. Hiện là bộ âm tạm; nhạc nền đang được soạn.</p>
	</fieldset>

	<fieldset>
		<legend class="panel-title text-lg">Hiển thị</legend>
		{#if fullscreen.supported}
			<div class="row mt-3">
				<span>Toàn màn hình</span>
				<button type="button" class="btn btn-ghost ml-auto h-10 px-3 text-sm" aria-pressed={fullscreen.active} onclick={() => fullscreen.toggle()} data-fullscreen>
					<Icon name={fullscreen.active ? 'shrink' : 'expand'} size={16} />
					{fullscreen.active ? 'Thoát' : 'Bật'}
				</button>
			</div>
		{/if}
		<p class="mt-3" id="renderer-label">Kết xuất</p>
		<div class="mt-2 grid grid-cols-2 gap-2" role="radiogroup" aria-labelledby="renderer-label">
			{#each rendererOptions as o (o.value)}
				<button
					type="button"
					role="radio"
					aria-checked={settings.rendererChoice === o.value}
					class="opt"
					class:is-on={settings.rendererChoice === o.value}
					title={o.hint}
					data-setting-renderer={o.value}
					onclick={() => settings.setRenderer(o.value)}
				>
					{o.label}
				</button>
			{/each}
		</div>
		<p class="mt-2 text-xs text-mist-500" data-renderer-note>
			{#if settings.fallback3d}
				Đã chuyển về 2D: {fallbackLabel[settings.fallback3d.reason]}. Chọn lại “3D thử nghiệm” để thử lần nữa.
			{:else if settings.rendererRequested === '3d' || settings.rendererRequested === '2d'}
				Đường dẫn đang buộc dùng {settings.rendererRequested.toUpperCase()}.
			{:else}
				3D đang thử nghiệm: chỉ Tử Tiêu Cung, tốn pin hơn 2D.
			{/if}
		</p>
	</fieldset>
</div>

<style>
	.opt {
		min-height: 2.5rem;
		font-weight: 600;
		color: var(--color-mist-300);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 30%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 40%, transparent);
	}
	.opt.is-on {
		color: var(--color-ink-950);
		background: var(--color-jade-300);
		border-color: var(--color-jade-300);
	}
	.row {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		min-height: 2.75rem;
		border-bottom: 1px dashed color-mix(in oklab, var(--color-gold-500) 12%, transparent);
	}
</style>

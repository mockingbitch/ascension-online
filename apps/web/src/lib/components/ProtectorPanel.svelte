<!--
	Hộ pháp (Phase 3.4): invite online fellow faction members to guard your tribulation, and guard theirs. Everything is
	the server's state (GET /protector, replaced after each action); whether a protector "counts" is decided by the
	server at attempt time (online + same faction + accepted, at most maxCounted). Refreshes every 30 s
	while mounted, and refreshes the breakthrough preview so the odds breakdown follows.
-->
<script lang="ts">
	import { ApiError, api } from '$lib/api/client';
	import type { CatalogueIndex } from '$lib/game/catalogue';
	import { formatBp } from '$lib/game/format';
	import { game } from '$lib/game/store.svelte';
	import { toast } from '$lib/game/toasts.svelte';
	import type { ProtectorView } from '$lib/game/types';
	import { untrack } from 'svelte';
	import Icon from './Icon.svelte';

	interface Props {
		catalogue: CatalogueIndex;
		/** the viewer's preview shows a tribulation (protectors only matter then) */
		tribulation: boolean;
	}
	let { catalogue, tribulation }: Props = $props();

	let view = $state<ProtectorView | null>(null);
	let busy = $state(false);
	let failed = $state(false);

	async function load() {
		try {
			const before = view?.myInvite?.counted ?? -1;
			view = await api.protector();
			failed = false;
			if (view.myInvite && view.myInvite.counted !== before) void game.refreshSide(); // odds follow the counted protectors
		} catch {
			failed = true;
		}
	}

	// Poll while mounted (protectors come and go); runs once per mount.
	$effect(() => {
		untrack(() => void load()); // load reads `view`; never let that re-trigger this effect
		const t = setInterval(() => {
			if (document.visibilityState === 'visible') void load();
		}, 30_000);
		return () => clearInterval(t);
	});

	async function act(fn: () => Promise<ProtectorView>, ok: string) {
		busy = true;
		try {
			view = await fn();
			toast('ok', ok);
			void game.refreshSide();
		} catch (e) {
			toast('err', e instanceof ApiError ? e.message : 'Không gửi được, thử lại sau.');
		} finally {
			busy = false;
		}
	}

	const stageName = (id: string) => catalogue.stages.get(id)?.name ?? id;
	const fmtTime = (iso: string) => new Date(iso).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' });
</script>

<section aria-labelledby="hp-title" class="space-y-3">
	<h2 id="hp-title" class="panel-title">Hộ pháp đồng môn</h2>
	{#if !view}
		<p class="text-sm text-mist-500">{failed ? 'Chưa kết nối được với đồng môn.' : 'Đang tìm đồng môn…'}</p>
	{:else if !view.factionId}
		<p class="text-sm text-mist-300">Gia nhập một thế lực để đồng môn hộ pháp khi độ kiếp. <a href="/play/the-luc" class="text-jade-300 underline underline-offset-2">Thế Lực</a></p>
	{:else}
		<p class="text-xs text-mist-500">
			Mỗi đồng môn đang trực tuyến đã nhận lời: +{formatBp(view.perMemberBp)} tỉ lệ vượt kiếp, tối đa {view.maxCounted} người (+{formatBp(view.maxBp)}).
			Lời mời giữ {Math.round(view.inviteWindowSeconds / 60)} phút và dùng hết ở lần độ kiếp kế tiếp.
		</p>

		{#if view.myInvite}
			{@const inv = view.myInvite}
			<div class="box p-3">
				<p class="flex items-center gap-2 text-sm font-semibold text-gold-300">
					<Icon name="sect" size={16} /> Đang mời hộ pháp · tới {fmtTime(inv.expiresAt)}
				</p>
				{#if inv.protectors.length}
					<ul class="mt-2 space-y-1 text-sm">
						{#each inv.protectors as pr (pr.characterId)}
							<li class="flex items-center gap-2">
								<span class="dot" class:on={pr.online} aria-hidden="true"></span>
								<span class="text-mist-100">{pr.name}</span>
								<span class="ml-auto text-xs" class:text-jade-300={pr.counts} class:text-mist-500={!pr.counts}>
									{pr.counts ? 'được tính' : !pr.online ? 'ngoại tuyến' : !pr.sameFaction ? 'đã rời thế lực' : 'vượt số người'}
								</span>
							</li>
						{/each}
					</ul>
					<p class="mt-2 text-xs text-mist-300">Đang tính <b class="text-mist-100">{inv.counted}</b>/{view.maxCounted} hộ pháp vào tỉ lệ.</p>
				{:else}
					<p class="mt-1 text-sm text-mist-500">Chưa có đồng môn nhận lời. Báo cho họ ở kênh thế lực hoặc chờ một lúc.</p>
				{/if}
			</div>
		{:else if view.eligible}
			<button type="button" class="btn btn-jade h-11 w-full" disabled={busy || !tribulation} onclick={() => void act(() => api.protectorInvite(), 'Đã mời đồng môn hộ pháp.')}>
				<Icon name="sect" size={16} /> Mời hộ pháp
			</button>
		{:else}
			<p class="text-xs text-mist-500">Chỉ mời hộ pháp khi đứng trước lôi kiếp (tầng cuối của cảnh giới có thiên kiếp).</p>
		{/if}

		<div>
			<p class="eyebrow">Đồng môn đang trực tuyến ({view.onlineMembers.length})</p>
			{#if view.onlineMembers.length}
				<p class="mt-1 text-sm text-mist-300">
					{#each view.onlineMembers.slice(0, 8) as m, i (m.characterId)}{i ? ', ' : ''}<span class="text-mist-100">{m.name}</span>{/each}{view.onlineMembers.length > 8 ? '…' : ''}
				</p>
			{:else}
				<p class="mt-1 text-sm text-mist-500">Chưa thấy ai.</p>
			{/if}
		</div>

		<div>
			<p class="eyebrow">Đồng môn cần hộ pháp · còn {view.assistsLeft}/{view.dailyAssists} lần hôm nay</p>
			{#if view.openInvites.length}
				<ul class="mt-2 space-y-2">
					{#each view.openInvites as o (o.inviteId)}
						<li class="box flex items-center gap-2 p-2 text-sm">
							<div class="min-w-0">
								<p class="truncate text-mist-100">{o.name}</p>
								<p class="truncate text-xs text-mist-500">{stageName(o.stageId)} · {o.protectors} hộ pháp · tới {fmtTime(o.expiresAt)}</p>
							</div>
							{#if o.accepted}
								<span class="ml-auto flex shrink-0 items-center gap-1 text-xs text-gold-300"><Icon name="check" size={14} /> Đang hộ pháp</span>
							{:else}
								<button
									type="button"
									class="btn btn-gold ml-auto h-10 shrink-0 px-3 text-sm"
									disabled={busy || view.assistsLeft <= 0}
									onclick={() => void act(() => api.protectorAccept(o.inviteId), `Đã nhận hộ pháp cho ${o.name}. Giữ trang mở để được tính là trực tuyến.`)}
								>
									Hộ pháp
								</button>
							{/if}
						</li>
					{/each}
				</ul>
			{:else}
				<p class="mt-1 text-sm text-mist-500">Chưa có lời mời nào.</p>
			{/if}
			<p class="mt-2 text-xs text-mist-500">Hộ pháp được tính khi đồng môn độ kiếp: nhận điểm duyên (có hạn mức ngày). Không tốn gì của bạn.</p>
		</div>
	{/if}
</section>

<style>
	.box {
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 22%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 50%, transparent);
	}
	.dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: var(--color-ink-600);
		flex-shrink: 0;
	}
	.dot.on {
		background: var(--color-jade-300);
	}
</style>

<!-- Faction emblems (64×64, currentColor), one motif per faction from the story briefs (keys = permanent legacy slugs,
     names in content/catalog/factions.yaml). Generic motifs only — no religious or worship symbols. -->
<script lang="ts">
	import { art } from '$lib/game/art';

	let { id, size = 64, class: cls = '' }: { id: string; size?: number; class?: string } = $props();
	const img = $derived(art(`factions/${id}`)); // raster badge slot, SVG emblem otherwise
</script>

{#if img}
<img src={img.src} srcset={img.srcset} alt="" width={size} height={size} class={['object-contain', cls]} draggable="false" />
{:else}
<svg viewBox="0 0 64 64" width={size} height={size} class={cls} fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
	<circle cx="32" cy="32" r="29" />
	<circle cx="32" cy="32" r="25" stroke-width="0.8" opacity="0.6" />
	{#if id === 'huyen_khung_mon'}
		<!-- Tam Thanh: one vault, three branches of the way -->
		<path d="M14 40a18 18 0 0 1 36 0" />
		<path d="M32 22v24M22 30l10 8 10-8" />
		<circle cx="22" cy="30" r="2.5" fill="currentColor" /><circle cx="32" cy="22" r="2.5" fill="currentColor" /><circle cx="42" cy="30" r="2.5" fill="currentColor" />
		<path d="M16 46h32" />
	{:else if id === 'kim_sa_mon'}
		<!-- Tây Phương: low sun over dunes, falling grains -->
		<circle cx="32" cy="26" r="7" />
		<path d="M12 44c6-6 12-6 18 0s12 6 22-2" /><path d="M14 50c8-4 14-4 20 0s10 4 16 0" />
		<path d="M24 16v2M32 13v2M40 16v2" />
	{:else if id === 'thuong_lan_dinh'}
		<!-- Tiên Đình: floating island above three waves -->
		<path d="M22 26h20l-4 6H26Z" /><path d="M28 26v-6h8v6M26 20h12" />
		<path d="M12 40c4-3 8-3 12 0s8 3 12 0 8-3 12 0" /><path d="M14 46c4-3 8-3 12 0s8 3 12 0 8-3 10-1" />
	{:else if id === 'tinh_ha_yeu_dinh'}
		<!-- Thiên Đình Yêu Tộc: crescent cradling a natal star, river of stars -->
		<path d="M40 18a14 14 0 1 0 6 20 11 11 0 1 1-6-20Z" />
		<path d="m30 24 1.8 4 4.2.4-3.2 2.8 1 4.2L30 33l-3.8 2.4 1-4.2-3.2-2.8 4.2-.4Z" fill="currentColor" />
		<circle cx="18" cy="44" r="1.2" fill="currentColor" /><circle cx="24" cy="48" r="1" fill="currentColor" /><circle cx="44" cy="46" r="1.4" fill="currentColor" />
	{:else if id === 'vu_toc'}
		<!-- Vu Tộc: totem pillar with drum and blood marks -->
		<path d="M28 14h8v36h-8Z" /><path d="M24 20h16M24 44h16" />
		<path d="M30 26l2 4 2-4M30 34l2 4 2-4" />
		<path d="M16 30c2-4 6-4 8 0M40 30c2-4 6-4 8 0" />
	{:else if id === 'thu_nhat_vien' || id === 'lap_cuc_vien' || id === 'van_luu_vien'}
		<!-- Tam Thanh split: one arc of the vault per giáo (Nhân Giáo · Xiển Giáo · Tiệt Giáo) -->
		{#if id === 'thu_nhat_vien'}
			<path d="M18 38a14 14 0 0 1 28 0" /><circle cx="32" cy="38" r="3" fill="currentColor" />
		{:else if id === 'lap_cuc_vien'}
			<path d="M14 40a18 18 0 0 1 36 0" /><path d="M32 22v20M26 42h12" />
		{:else}
			<path d="M10 42a22 22 0 0 1 44 0" /><path d="M18 46c4-3 8-3 12 0s8 3 12 0 4-2 6-1" />
		{/if}
		<path d="M16 48h32" />
	{:else if id === 'kim_sa_giao'}
		<!-- Tây Phương Giáo: the Tây Phương dunes and sun, with a four-point star of the new creed -->
		<circle cx="32" cy="28" r="6" />
		<path d="M12 44c6-6 12-6 18 0s12 6 22-2" /><path d="M14 50c8-4 14-4 20 0s10 4 16 0" />
		<path d="M32 12l1.5 3.5L37 17l-3.5 1.5L32 22l-1.5-3.5L27 17l3.5-1.5Z" fill="currentColor" />
	{:else if id === 'nhan_toc'}
		<!-- Nhân Tộc: a figure shaped from earth, standing on a field line -->
		<circle cx="32" cy="22" r="5" /><path d="M32 27v13M24 32l8 3 8-3M26 50l6-10 6 10" /><path d="M14 50h36" />
	{:else}
		<path d="M24 32h16M32 24v16" />
	{/if}
</svg>
{/if}

<!--
	A title (cosmetics.yaml title:*) as a small plaque styled by rarity: phàm/linh a plain lacquer tablet, huyền a double
	rim, địa corner brackets, thiên a gold-leaf face, tiên gold + violet glow — and a light glint across the higher
	ones at effects full. Text stays readable (mist/gold on ink). Looks only, no gameplay effect.
-->
<script lang="ts">
	import type { Rarity } from '$lib/game/types';

	interface Props {
		name: string;
		rarity?: Rarity;
		size?: 'sm' | 'md';
		/** long names wrap (cards) instead of ellipsis */
		wrap?: boolean;
		/** a square seal (icon-sized: initials) */
		seal?: boolean;
		class?: string;
	}
	let { name, rarity = 'linh', size = 'md', wrap = false, seal = false, class: cls = '' }: Props = $props();
</script>

<span class="plaque font-display r-{rarity} {size} {cls}" class:wrap class:seal style:--r="var(--color-rarity-{rarity})" data-title-rarity={rarity}>
	<span class="txt">{name}</span>
</span>

<style>
	.plaque {
		position: relative;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		max-width: 100%;
		padding: 0.12rem 0.7rem;
		font-weight: 700;
		letter-spacing: 0.04em;
		line-height: 1.3;
		color: color-mix(in oklab, var(--r) 70%, var(--color-mist-100));
		border: 1px solid color-mix(in oklab, var(--r) 55%, transparent);
		background: linear-gradient(180deg, color-mix(in oklab, var(--r) 16%, var(--color-ink-900)), var(--color-ink-950));
		overflow: hidden;
		white-space: nowrap;
	}
	.plaque.md {
		font-size: 0.95rem;
	}
	.plaque.sm {
		font-size: 0.82rem;
		padding: 0.08rem 0.55rem;
	}
	.plaque.wrap {
		white-space: normal;
		text-align: center;
	}
	.plaque.seal {
		width: 78%;
		aspect-ratio: 1;
		padding: 0;
		font-size: 1.1rem;
	}
	.txt {
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.r-huyen,
	.r-dia,
	.r-thien,
	.r-tien {
		box-shadow: inset 0 0 0 2px var(--color-ink-950), inset 0 0 0 3px color-mix(in oklab, var(--r) 45%, transparent);
	}
	/* corner brackets */
	.r-dia::before,
	.r-thien::before,
	.r-tien::before {
		content: '';
		position: absolute;
		inset: 2px;
		pointer-events: none;
		background:
			linear-gradient(var(--r), var(--r)) left top / 6px 1.5px no-repeat,
			linear-gradient(var(--r), var(--r)) left top / 1.5px 6px no-repeat,
			linear-gradient(var(--r), var(--r)) right top / 6px 1.5px no-repeat,
			linear-gradient(var(--r), var(--r)) right top / 1.5px 6px no-repeat,
			linear-gradient(var(--r), var(--r)) left bottom / 6px 1.5px no-repeat,
			linear-gradient(var(--r), var(--r)) left bottom / 1.5px 6px no-repeat,
			linear-gradient(var(--r), var(--r)) right bottom / 6px 1.5px no-repeat,
			linear-gradient(var(--r), var(--r)) right bottom / 1.5px 6px no-repeat;
	}
	.r-thien,
	.r-tien {
		color: var(--color-ink-950);
		border-color: var(--color-gold-300);
		background: linear-gradient(180deg, var(--color-gold-300), var(--color-gold-500) 60%, var(--color-gold-700));
		text-shadow: 0 1px 0 color-mix(in oklab, var(--color-gold-300) 60%, transparent);
	}
	.r-thien::before,
	.r-tien::before {
		--r: var(--color-ink-950);
	}
	.r-tien {
		box-shadow:
			inset 0 0 0 2px var(--color-gold-700),
			0 0 10px color-mix(in oklab, var(--color-spirit-violet) 70%, transparent),
			0 0 18px color-mix(in oklab, var(--color-spirit-violet) 35%, transparent);
	}
	/* glint across the gold faces (full only) */
	.r-thien::after,
	.r-tien::after,
	.r-dia::after {
		content: '';
		position: absolute;
		inset: -20% auto -20% -40%;
		width: 30%;
		pointer-events: none;
		background: linear-gradient(100deg, transparent, color-mix(in oklab, var(--color-mist-100) 55%, transparent), transparent);
		transform: translateX(-100%) skewX(-12deg);
		opacity: 0;
	}
	:global(:root[data-fx='full']) .r-thien::after,
	:global(:root[data-fx='full']) .r-tien::after,
	:global(:root[data-fx='full']) .r-dia::after {
		opacity: 1;
		animation: plaque-glint 4.8s var(--ease-qi) infinite;
	}
	@keyframes plaque-glint {
		0%,
		60% {
			transform: translateX(-100%) skewX(-12deg);
		}
		100% {
			transform: translateX(560%) skewX(-12deg);
		}
	}
</style>

<script lang="ts" module>
	/** Original line icons, 24×24, stroke = currentColor. Decorative unless `label` is given. */
	const PATHS = {
		// Lotus — tu luyện
		lotus: 'M12 19c-4 0-7.5-2-8.5-5 2.5-.6 5 .2 6.6 2M12 19c4 0 7.5-2 8.5-5-2.5-.6-5 .2-6.6 2M12 19c-2.2-1.6-3.2-4-3.2-6.6C8.8 9.6 10.2 7 12 5c1.8 2 3.2 4.6 3.2 7.4 0 2.6-1 5-3.2 6.6ZM6.5 10.5c-.4-1.4-.4-2.8.1-4.2 1.3.4 2.4 1.1 3.3 2.1M17.5 10.5c.4-1.4.4-2.8-.1-4.2-1.3.4-2.4 1.1-3.3 2.1',
		// Mountain gate — động phủ
		cave: 'M3 20h18M5 20V9l7-5 7 5v11M9 20v-5a3 3 0 0 1 6 0v5M3.5 10 12 4l8.5 6',
		// Scroll — công pháp
		scroll: 'M7 4h11a2 2 0 0 1 2 2v1h-4M7 4a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h9a2 2 0 0 0 2-2V7M7 4a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2M11 9h4M11 12h4M11 15h3',
		// Gourd with pill — đan dược
		pill: 'M12 3v2M10 5h4M10.2 5.2C8.4 6 8 7.6 9 9c-2.8 1-4.5 3.4-4.5 6a7.5 6 0 0 0 15 0c0-2.6-1.7-5-4.5-6 1-1.4.6-3-1.2-3.8M9 15.5h6',
		// Banner — tông môn
		sect: 'M6 21V3M6 4h11l-2.5 4L17 12H6M10 6.5v3',
		// Stele — thiên bảng
		rank: 'M8 21h8M10 21v-3h4v3M7 18h10M8 18V6.5L12 3l4 3.5V18M10.5 9h3M10.5 12h3',
		// Pouch — túi đồ
		bag: 'M9 4h6l-1.5 3h-3L9 4ZM10.5 7C6.5 8.5 4.5 12 5 16c.4 3 3 5 7 5s6.6-2 7-5c.5-4-1.5-7.5-5.5-9M9.5 13h5',
		// Faceted spirit stone
		stone: 'M7 4h10l4 5-9 11L3 9l4-5ZM3 9h18M9 9l3 11 3-11M7 4l2 5M17 4l-2 5M12 4v5',
		// Talisman paper
		talisman: 'M7 3h10v18l-5-3-5 3V3ZM10 7h4M12 7v8M10 11h4M10.5 14l1.5 1.5 1.5-1.5',
		// Seal (Tiên Duyên)
		seal: 'M12 3 14 5h3v3l2 2-2 2v3h-3l-2 2-2-2H7v-3l-2-2 2-2V5h3l2-2ZM9.5 10h5M12 10v4.5M10 14.5h4',
		gift: 'M4 10h16v3H4zM5 13h14v8H5zM12 10v11M12 10c-1-3-4.5-4-5.5-2.5S8 10 12 10Zm0 0c1-3 4.5-4 5.5-2.5S16 10 12 10Z',
		// Folded map — thế giới
		map: 'M3 6.5 9 4l6 2.5L21 4v13.5L15 20l-6-2.5L3 20V6.5ZM9 4v13.5M15 6.5V20',
		// Robed figure — nhân vật
		person: 'M12 3.5a3 3 0 1 1 0 6 3 3 0 0 1 0-6ZM5.5 20.5c.6-4.6 3.2-7 6.5-7s5.9 2.4 6.5 7M9.5 14l2.5 3 2.5-3',
		// Two hearts entwined — đạo lữ
		heart: 'M12 20s-7-4.3-7-9.5A3.8 3.8 0 0 1 12 8a3.8 3.8 0 0 1 7 2.5C19 15.7 12 20 12 20Z',
		// Moon gate — bí cảnh
		portal: 'M5 21V11a7 7 0 0 1 14 0v10M3 21h18M9 21v-8a3 3 0 0 1 6 0v8',
		// Temple bell — thông báo
		bell: 'M12 3.5c3.3 0 5.5 2.6 5.5 6v3.5l1.5 3H5l1.5-3V9.5c0-3.4 2.2-6 5.5-6ZM10 19a2 2 0 0 0 4 0',
		// Speech scroll — truyền âm (chat)
		chat: 'M4 5h16v11H9l-4 4v-4H4V5ZM8 9h8M8 12h5',
		// Gear — tùy chỉnh
		gear: 'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6ZM12 2.5v3M12 18.5v3M2.5 12h3M18.5 12h3M5.3 5.3l2.1 2.1M16.6 16.6l2.1 2.1M5.3 18.7l2.1-2.1M16.6 7.4l2.1-2.1',
		// Pennant — báo cáo
		flag: 'M6 21V4M6 4h11l-2 4 2 4H6',
		lock: 'M7 11V8a5 5 0 0 1 10 0v3M5 11h14v10H5zM12 15v2',
		check: 'm5 12.5 4.5 4.5L19 7.5',
		cross: 'M6 6l12 12M18 6 6 18',
		bolt: 'M13 2 5 13.5h6L10 22l8.5-12H12.5L13 2Z',
		clock: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18ZM12 7v5l3 2',
		spark: 'M12 3c.6 4.2 2.8 6.4 7 7-4.2.6-6.4 2.8-7 7-.6-4.2-2.8-6.4-7-7 4.2-.6 6.4-2.8 7-7Z',
		arrow: 'M5 12h14M13 6l6 6-6 6',
		exit: 'M14 4h5v16h-5M10 8l-4 4 4 4M6 12h10'
	} as const;
	export type IconName = keyof typeof PATHS;
</script>

<script lang="ts">
	interface Props {
		name: IconName;
		size?: number;
		label?: string;
		class?: string;
		stroke?: number;
	}
	let { name, size = 20, label, class: cls = '', stroke = 1.5 }: Props = $props();
</script>

<svg
	viewBox="0 0 24 24"
	width={size}
	height={size}
	fill="none"
	stroke="currentColor"
	stroke-width={stroke}
	stroke-linecap="round"
	stroke-linejoin="round"
	class="shrink-0 {cls}"
	role={label ? 'img' : undefined}
	aria-label={label}
	aria-hidden={label ? undefined : 'true'}
>
	<path d={PATHS[name]} />
</svg>

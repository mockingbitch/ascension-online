<!--
	Ambient world behind every screen: night sky, moon halo, three layers of ink-wash peaks with a distant
	celestial pavilion, and two slow mist bands. One fixed SVG; only the mist bands animate (transform only).
-->
<div class="backdrop" aria-hidden="true">
	<svg viewBox="0 0 1600 900" preserveAspectRatio="xMidYMax slice" class="size-full">
		<defs>
			<linearGradient id="wb-sky" x1="0" y1="0" x2="0" y2="1">
				<stop offset="0" stop-color="var(--color-ink-950)" />
				<stop offset="0.55" stop-color="var(--color-ink-850)" />
				<stop offset="1" stop-color="var(--color-ink-800)" />
			</linearGradient>
			<radialGradient id="wb-moon" cx="0.5" cy="0.5" r="0.5">
				<stop offset="0" stop-color="var(--color-gold-300)" stop-opacity="0.38" />
				<stop offset="0.18" stop-color="var(--color-gold-300)" stop-opacity="0.12" />
				<stop offset="1" stop-color="var(--color-gold-300)" stop-opacity="0" />
			</radialGradient>
			<linearGradient id="wb-far" x1="0" y1="0" x2="0" y2="1">
				<stop offset="0" stop-color="var(--color-ink-600)" stop-opacity="0.55" />
				<stop offset="1" stop-color="var(--color-ink-800)" stop-opacity="0.2" />
			</linearGradient>
			<linearGradient id="wb-mid" x1="0" y1="0" x2="0" y2="1">
				<stop offset="0" stop-color="var(--color-ink-700)" />
				<stop offset="1" stop-color="var(--color-ink-850)" />
			</linearGradient>
			<linearGradient id="wb-near" x1="0" y1="0" x2="0" y2="1">
				<stop offset="0" stop-color="var(--color-ink-850)" />
				<stop offset="1" stop-color="var(--color-ink-950)" />
			</linearGradient>
			<radialGradient id="wb-vignette" cx="0.5" cy="0.45" r="0.75">
				<stop offset="0.55" stop-color="var(--color-ink-950)" stop-opacity="0" />
				<stop offset="1" stop-color="var(--color-ink-950)" stop-opacity="0.85" />
			</radialGradient>
			<filter id="wb-soft"><feGaussianBlur stdDeviation="2.5" /></filter>
			<filter id="wb-mist" x="-20%" y="-200%" width="140%" height="500%"><feGaussianBlur stdDeviation="18" /></filter>
		</defs>

		<rect width="1600" height="900" fill="url(#wb-sky)" />
		<circle cx="1210" cy="190" r="260" fill="url(#wb-moon)" />
		<circle cx="1210" cy="190" r="34" fill="var(--color-gold-300)" opacity="0.16" />

		<!-- far peaks + pavilion -->
		<g filter="url(#wb-soft)">
			<path
				fill="url(#wb-far)"
				d="M0 560 Q90 470 150 500 Q200 380 240 330 Q262 390 300 470 Q360 430 410 455 Q470 350 520 260 Q545 330 580 420
				   Q640 380 700 410 Q760 300 800 250 Q830 320 870 400 Q950 360 1010 420 Q1070 330 1120 290 Q1150 350 1190 430
				   Q1260 390 1320 420 Q1380 320 1430 280 Q1460 360 1500 440 Q1550 420 1600 450 V900 H0Z"
			/>
		</g>
		<g fill="var(--color-ink-600)" opacity="0.7">
			<!-- tiny tiered pavilion on the tallest far peak -->
			<path d="M786 252h28l-4 5h-20zM790 257h20v6h-20zM781 263h38l-5 6h-28zM786 269h28v8h-28zM778 277h44l-4 4h-36z" />
		</g>

		<!-- mid peaks: steep, ink-wash needles -->
		<path
			fill="url(#wb-mid)"
			d="M0 650 Q70 600 120 620 Q160 520 195 455 Q215 520 245 600 Q300 570 350 590 Q395 500 430 445 Q452 515 485 590
			   Q560 560 620 600 Q700 610 760 630 Q830 600 900 610 Q960 520 1000 470 Q1022 540 1060 600 Q1120 575 1180 595
			   Q1240 500 1275 450 Q1296 520 1330 590 Q1400 570 1460 590 Q1530 540 1600 560 V900 H0Z"
		/>
		<!-- near ridge -->
		<path
			fill="url(#wb-near)"
			d="M0 760 Q120 700 240 730 Q330 690 420 720 Q520 750 640 735 Q760 700 880 735 Q990 765 1100 730 Q1220 690 1330 725
			   Q1450 760 1600 720 V900 H0Z"
		/>

		<!-- mist bands -->
		<g class="mist mist-a">
			<ellipse cx="500" cy="640" rx="560" ry="26" fill="var(--color-mist-100)" opacity="0.07" filter="url(#wb-mist)" />
		</g>
		<g class="mist mist-b">
			<ellipse cx="1150" cy="700" rx="620" ry="30" fill="var(--color-mist-100)" opacity="0.06" filter="url(#wb-mist)" />
		</g>

		<rect width="1600" height="900" fill="url(#wb-vignette)" />
	</svg>
</div>

<style>
	.backdrop {
		position: fixed;
		inset: 0;
		z-index: -10;
		pointer-events: none;
	}
	.mist {
		will-change: transform;
		animation: drift 46s ease-in-out infinite alternate;
	}
	.mist-b {
		animation-duration: 60s;
		animation-direction: alternate-reverse;
	}
	@keyframes drift {
		from {
			transform: translateX(-60px);
		}
		to {
			transform: translateX(60px);
		}
	}
</style>

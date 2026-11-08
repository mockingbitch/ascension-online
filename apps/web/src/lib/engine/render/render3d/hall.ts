// Procedural Tử Tiêu Cung (GĐ7 spike): no downloaded assets — a floating floor platform, pillars, the back facade
// with its eave silhouette, the three-tier dais, Hồng Quân as a stylised robed figure with a gold glow, listener
// cushions at the def's seat anchors, and cloud sprites. Every object that an anchor names is BUILT at that anchor's
// 3D point (mapping.ts), so UI overlays land on it. Colours only from the token palette. Kept small: ~20 draw calls,
// a few thousand triangles (budget: ≤ 120 calls, ≤ 150k triangles).
import {
	AdditiveBlending,
	CanvasTexture,
	CapsuleGeometry,
	CircleGeometry,
	Color,
	ConeGeometry,
	CylinderGeometry,
	DirectionalLight,
	Fog,
	Group,
	HemisphereLight,
	InstancedMesh,
	LatheGeometry,
	type Material,
	Mesh,
	MeshBasicMaterial,
	MeshStandardMaterial,
	Object3D,
	PlaneGeometry,
	PointLight,
	Scene,
	SphereGeometry,
	Sprite,
	SpriteMaterial,
	SRGBColorSpace,
	Texture,
	TorusGeometry,
	BoxGeometry,
	Vector2,
	type BufferGeometry
} from 'three';
import type { Quality } from '../../fx/particles';
import type { SceneDef } from '../../scene/types';
import { DAIS_TOP, HALL } from './layout';
import { anchorToWorld, type CameraPose, type Vec3 } from './mapping';
import { mix, type Palette, type Rgb } from './palette';

const col = (c: Rgb) => new Color().setRGB(c[0], c[1], c[2], SRGBColorSpace);

export interface HallBuild {
	scene: Scene;
	/** the procedural architecture (replaced by a model.glb when one ships) */
	architecture: Group;
	/** Hồng Quân placeholder (replaced by characters/hong-quan.glb when one ships) */
	sage: Group;
	/** where the sage stands (the `hong_quan` anchor in 3D) */
	sageAt: Vec3;
	key: DirectionalLight;
	/** per-frame ambient motion (clouds, glow); `t` = ms */
	update(t: number): void;
	setQuality(q: Quality): void;
	dispose(): void;
}

/** Soft round puff for cloud sprites, drawn once on a small canvas (tinted by the sprite colour). */
function puffTexture(): Texture {
	const c = document.createElement('canvas');
	c.width = c.height = 64;
	const g = c.getContext('2d')!;
	// greyscale alpha mask only: the colour comes from the material (a token)
	const grad = g.createRadialGradient(32, 32, 2, 32, 32, 32);
	grad.addColorStop(0, 'rgba(255,255,255,0.9)');
	grad.addColorStop(0.5, 'rgba(255,255,255,0.35)');
	grad.addColorStop(1, 'rgba(255,255,255,0)');
	g.fillStyle = grad;
	g.fillRect(0, 0, 64, 64);
	const t = new CanvasTexture(c);
	t.colorSpace = SRGBColorSpace;
	return t;
}

/** Deterministic pseudo-random (layout of clouds must not change between mounts). */
function rng(seed: number) {
	let s = seed >>> 0;
	return () => {
		s = (s * 1664525 + 1013904223) >>> 0;
		return s / 2 ** 32;
	};
}

export function buildHall(def: SceneDef, pose: CameraPose, pal: Palette, quality: Quality): HallBuild {
	const scene = new Scene();
	const geos: BufferGeometry[] = [];
	const mats: Material[] = [];
	const texs: Texture[] = [];
	const g = <T extends BufferGeometry>(x: T) => (geos.push(x), x);
	const m = <T extends Material>(x: T) => (mats.push(x), x);
	const std = (c: Rgb, o: { rough?: number; metal?: number; emissive?: Rgb; ei?: number } = {}) =>
		m(
			new MeshStandardMaterial({
				color: col(c),
				roughness: o.rough ?? 0.75,
				metalness: o.metal ?? 0,
				...(o.emissive ? { emissive: col(o.emissive), emissiveIntensity: o.ei ?? 1 } : {})
			})
		);
	const mesh = (geo: BufferGeometry, mat: Material, at: [number, number, number], shadow = true) => {
		const x = new Mesh(geo, mat);
		x.position.set(...at);
		x.castShadow = shadow;
		x.receiveShadow = true;
		return x;
	};

	// ── palette mixes (all from tokens) ──
	const fogC = mix(pal['ink-900'], pal['spirit-violet'], 0.18);
	const floorC = mix(pal['ink-800'], pal['spirit-violet'], 0.12);
	const lacquer = mix(pal['cinnabar-500'], pal['ink-950'], 0.45);
	const wallC = mix(pal['ink-900'], pal['spirit-violet'], 0.22);
	const roofC = mix(pal['ink-950'], pal['jade-700'], 0.25);
	const robe = mix(pal['mist-300'], pal['spirit-violet'], 0.3);
	const cushionC = mix(pal['spirit-violet'], pal['ink-800'], 0.55);

	scene.fog = new Fog(col(fogC), 20, 52);

	// ── light: a soft gold key, violet sky fill, a warm glow on the sage ──
	scene.add(new HemisphereLight(col(mix(pal['mist-300'], pal['spirit-violet'], 0.45)), col(pal['ink-950']), 0.8));
	const key = new DirectionalLight(col(pal['gold-300']), 2.0);
	key.position.set(-7, 15, 9);
	key.target.position.set(0, 0, -6);
	key.shadow.mapSize = new Vector2(1024, 1024);
	key.shadow.camera.left = -12;
	key.shadow.camera.right = 12;
	key.shadow.camera.top = 14;
	key.shadow.camera.bottom = -14;
	key.shadow.camera.near = 1;
	key.shadow.camera.far = 45;
	key.shadow.bias = -0.0008;
	scene.add(key, key.target);

	const architecture = new Group();
	architecture.name = 'architecture';
	scene.add(architecture);

	// ── floating platform + gold edge + aisle runner ──
	const F = HALL.floor;
	const depth = F.zTo - F.zFrom;
	const zMid = (F.zTo + F.zFrom) / 2;
	architecture.add(mesh(g(new BoxGeometry(F.halfW * 2, F.thick, depth)), std(floorC, { rough: 0.6 }), [0, -F.thick / 2, zMid], false));
	const trim = std(pal['gold-500'], { rough: 0.35, metal: 0.6 });
	const edgeGeo = g(new BoxGeometry(F.halfW * 2 + 0.2, 0.12, 0.2));
	architecture.add(mesh(edgeGeo, trim, [0, 0.02, F.zTo], false));
	const sideGeo = g(new BoxGeometry(0.2, 0.12, depth));
	architecture.add(mesh(sideGeo, trim, [-F.halfW, 0.02, zMid], false), mesh(sideGeo, trim, [F.halfW, 0.02, zMid], false));
	const runnerMat = std(mix(cushionC, pal['ink-950'], 0.35), { rough: 0.9 });
	const runner = mesh(g(new PlaneGeometry(1.4, F.zTo - 2 - (F.zFrom + 6))), runnerMat, [0, 0.01, (F.zTo - 2 + F.zFrom + 6) / 2], false);
	runner.rotation.x = -Math.PI / 2;
	architecture.add(runner);

	// ── back facade + eave silhouette (upturned corners) + the gold "đạo luân" disc behind the sage ──
	const Fa = HALL.facade;
	architecture.add(mesh(g(new BoxGeometry(Fa.halfW * 2, Fa.height, 0.6)), std(wallC, { rough: 0.85 }), [0, Fa.height / 2, Fa.z], false));
	const eave = std(roofC, { rough: 0.7 });
	const eaveBox = mesh(g(new BoxGeometry(Fa.halfW * 2 + 1.6, 0.35, 1.6)), eave, [0, Fa.height + 0.1, Fa.z + 0.5]);
	eaveBox.rotation.x = 0.18;
	architecture.add(eaveBox);
	architecture.add(mesh(g(new BoxGeometry(Fa.halfW * 2 + 1, 0.12, 0.12)), trim, [0, Fa.height - 0.12, Fa.z + 1.15], false));
	const hornGeo = g(new ConeGeometry(0.22, 1.3, 8));
	for (const s of [-1, 1]) {
		const horn = mesh(hornGeo, eave, [s * (Fa.halfW + 0.9), Fa.height + 0.55, Fa.z + 1.1]);
		horn.rotation.z = -s * 0.75;
		architecture.add(horn);
	}
	const sageAt = anchorToWorld(pose, def.anchors.hong_quan ?? { x: 0.5, y: 0.27, z: DAIS_TOP });
	const halo = new Mesh(g(new CircleGeometry(3.1, 48)), m(new MeshBasicMaterial({ color: col(mix(pal['gold-300'], wallC, 0.72)), fog: true })));
	halo.position.set(sageAt.x, 3.3, Fa.z + 0.32);
	const haloRing = new Mesh(g(new TorusGeometry(3.15, 0.07, 6, 64)), trim);
	haloRing.position.copy(halo.position);
	architecture.add(halo, haloRing);

	// ── pillars (lacquer red with gold bands) ──
	const P = HALL.pillars;
	const pillarGeo = g(new CylinderGeometry(P.radius, P.radius * 1.08, P.height, 16));
	const bandGeo = g(new TorusGeometry(P.radius * 1.1, 0.07, 6, 20));
	const baseGeo = g(new CylinderGeometry(P.radius * 1.5, P.radius * 1.6, 0.35, 16));
	const pillarMat = std(lacquer, { rough: 0.45 });
	const baseMat = std(mix(pal['ink-700'], pal['mist-300'], 0.25));
	for (const s of [-1, 1])
		for (const z of P.z) {
			architecture.add(mesh(pillarGeo, pillarMat, [s * P.x, P.height / 2, z]));
			architecture.add(mesh(baseGeo, baseMat, [s * P.x, 0.17, z], false));
			for (const y of [1.1, P.height - 0.6]) {
				const b = mesh(bandGeo, trim, [s * P.x, y, z], false);
				b.rotation.x = Math.PI / 2;
				architecture.add(b);
			}
		}

	// ── dais: three tiers with gold rims, centred under the sage; two incense burners ──
	const tiers = [
		{ r: 3.0, y0: 0 },
		{ r: 2.3, y0: DAIS_TOP / 3 },
		{ r: 1.6, y0: (2 * DAIS_TOP) / 3 }
	];
	const tierMat = std(mix(pal['ink-700'], pal['spirit-violet'], 0.2), { rough: 0.55 });
	for (const t of tiers) {
		architecture.add(mesh(g(new CylinderGeometry(t.r, t.r + 0.08, DAIS_TOP / 3, 40)), tierMat, [sageAt.x, t.y0 + DAIS_TOP / 6, sageAt.z]));
		const rim = mesh(g(new TorusGeometry(t.r + 0.02, 0.05, 6, 48)), trim, [sageAt.x, t.y0 + DAIS_TOP / 3, sageAt.z], false);
		rim.rotation.x = Math.PI / 2;
		architecture.add(rim);
	}
	const burnerGeo = g(new CylinderGeometry(0.28, 0.2, 0.6, 12));
	const emberMat = m(new MeshBasicMaterial({ color: col(pal['gold-300']) }));
	const emberGeo = g(new SphereGeometry(0.12, 10, 8));
	for (const s of [-1, 1]) {
		architecture.add(mesh(burnerGeo, std(pal['gold-700'], { metal: 0.6, rough: 0.4 }), [sageAt.x + s * 3.8, 0.3, sageAt.z + 1.2]));
		architecture.add(mesh(emberGeo, emberMat, [sageAt.x + s * 3.8, 0.66, sageAt.z + 1.2], false));
	}

	// ── Hồng Quân placeholder: seated robe bell + torso capsule + head, gold halo ring, warm point light ──
	const sage = new Group();
	sage.name = 'sage';
	sage.position.set(sageAt.x, sageAt.y, sageAt.z);
	const robeMat = std(robe, { rough: 0.85, emissive: pal['spirit-violet'], ei: 0.05 });
	const bell = new LatheGeometry(
		[
			[0, 0],
			[1.45, 0],
			[1.5, 0.12],
			[1.2, 0.55],
			[0.7, 0.95],
			[0, 1.05]
		].map(([x, y]) => new Vector2(x!, y!)),
		28
	);
	sage.add(mesh(g(bell), robeMat, [0, 0, 0]));
	sage.add(mesh(g(new CapsuleGeometry(0.55, 0.9, 6, 16)), robeMat, [0, 1.55, 0]));
	const sash = mesh(g(new TorusGeometry(0.57, 0.06, 6, 24)), trim, [0, 1.25, 0], false);
	sash.rotation.x = Math.PI / 2;
	sage.add(sash);
	sage.add(mesh(g(new SphereGeometry(0.36, 20, 14)), std(mix(pal['mist-100'], pal['gold-300'], 0.3)), [0, 2.55, 0]));
	sage.add(mesh(g(new SphereGeometry(0.17, 12, 10)), std(mix(pal['ink-900'], pal['spirit-violet'], 0.2)), [0, 2.86, -0.3]));
	const ringMat = m(new MeshBasicMaterial({ color: col(pal['gold-300']), transparent: true, opacity: 0.85 }));
	const ring = new Mesh(g(new TorusGeometry(0.62, 0.04, 6, 40)), ringMat);
	ring.position.set(0, 2.6, -0.45);
	sage.add(ring);
	const glowMat = m(
		new SpriteMaterial({ color: col(pal['gold-300']), transparent: true, opacity: 0.4, blending: AdditiveBlending, depthWrite: false })
	);
	const puff = puffTexture();
	texs.push(puff);
	glowMat.map = puff;
	const glow = new Sprite(glowMat);
	glow.scale.set(5.2, 5.2, 1);
	glow.position.set(0, 1.9, -0.2);
	sage.add(glow);
	const sageLight = new PointLight(col(pal['gold-300']), 6, 10, 1.6);
	sageLight.position.set(0, 2.6, 1.6);
	sage.add(sageLight);
	scene.add(sage);

	// ── listener cushions at the def's seat anchors (2 instanced draws) ──
	const seats = Object.entries(def.anchors)
		.filter(([k]) => k.startsWith('seat.'))
		.map(([, a]) => anchorToWorld(pose, a));
	const C = HALL.cushion;
	const cushions = new InstancedMesh(g(new CylinderGeometry(C.radius, C.radius * 1.1, C.height, 20)), std(cushionC, { rough: 0.9 }), seats.length);
	const rims = new InstancedMesh(g(new TorusGeometry(C.radius * 1.02, 0.035, 5, 24)), trim, seats.length);
	const o = new Object3D();
	seats.forEach((p, i) => {
		o.position.set(p.x, C.height / 2, p.z);
		o.rotation.set(0, 0, 0);
		o.updateMatrix();
		cushions.setMatrixAt(i, o.matrix);
		o.position.y = C.height;
		o.rotation.x = Math.PI / 2;
		o.updateMatrix();
		rims.setMatrixAt(i, o.matrix);
	});
	cushions.castShadow = true;
	cushions.receiveShadow = true;
	scene.add(cushions, rims);

	// ── clouds: a sea of puffs around and below the platform, a few mist banks in the hall ──
	const cloudC = col(mix(pal['mist-300'], pal['spirit-violet'], 0.3));
	const cloudMat = m(new SpriteMaterial({ map: puff, color: cloudC, transparent: true, opacity: 0.22, depthWrite: false, fog: true }));
	const rand = rng(7);
	const clouds: { s: Sprite; speed: number; x0: number }[] = [];
	for (let i = 0; i < HALL.clouds.full; i++) {
		const s = new Sprite(cloudMat);
		const side = i % 2 ? 1 : -1;
		const inner = i % 6 === 5; // a low mist bank inside the hall, behind the seats
		const x = inner ? side * (2 + rand() * 4) : side * (F.halfW + 1 + rand() * 7);
		const y = inner ? 0.4 + rand() * 0.4 : -1.6 + rand() * 1.6;
		const z = inner ? F.zFrom + 2 + rand() * 4 : F.zFrom + rand() * (depth + 4);
		const k = inner ? 2.5 + rand() * 1.5 : 3.5 + rand() * 4;
		s.scale.set(k * 1.8, k, 1);
		s.position.set(x, y, z);
		s.renderOrder = 1;
		scene.add(s);
		clouds.push({ s, speed: (0.12 + rand() * 0.2) * (inner ? 0.4 : 1), x0: x });
	}

	const setQuality = (q: Quality) => {
		const n = HALL.clouds[q];
		clouds.forEach((c, i) => (c.s.visible = i < n));
		key.castShadow = q === 'full';
	};
	setQuality(quality);

	return {
		scene,
		architecture,
		sage,
		sageAt,
		key,
		update(t) {
			for (const c of clouds) if (c.s.visible) c.s.position.x = c.x0 + Math.sin((t / 1000) * c.speed + c.x0) * 1.2;
			const b = 0.5 + 0.5 * Math.sin((t / 6000) * 2 * Math.PI);
			glowMat.opacity = 0.28 + 0.14 * b;
			ring.rotation.z = (t / 1000) * 0.15;
		},
		setQuality,
		dispose() {
			// procedural resources, plus anything a slot loaded into the scene (glb meshes, their materials/maps)
			scene.traverse((o) => {
				const x = o as Partial<Mesh>;
				if (x.geometry) geos.push(x.geometry);
				for (const mt of Array.isArray(x.material) ? x.material : x.material ? [x.material] : []) {
					mats.push(mt);
					for (const v of Object.values(mt)) if (v instanceof Texture) texs.push(v);
				}
			});
			for (const x of new Set(geos)) x.dispose();
			for (const x of new Set(mats)) x.dispose();
			for (const x of new Set(texs)) x.dispose();
			key.shadow.map?.dispose();
			cushions.dispose();
			rims.dispose();
			scene.clear();
		}
	};
}

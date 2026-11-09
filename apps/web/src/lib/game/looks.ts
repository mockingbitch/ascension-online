// Character looks (content/catalog/appearances.yaml, decisions 2026-11-09 "Ngoại hình…" and "Bộ trang phục vẽ sẵn…"):
// pure helpers that turn a Look (body + outfit + pose, ids from the server) into what the SVG fallback draws and which
// raster art slots to try. Display only — the
// server validates every choice and serves the resolved look; nothing here decides ownership, path or cooldown.
import type { AppearanceCatalog, BodyDef, BodySex, Look, OutfitDef, PathId, SvgHair } from './types';

/** Everything CultivatorFigure needs to draw a look without raster art. */
export interface Figure {
	sex: BodySex;
	path: PathId;
	hair: SvgHair;
	beard: boolean;
	/** hair accessory id ('khong' = none) — the outfit's */
	acc: string;
	/** robe palette id → CSS token --color-robe-<id> — the outfit's */
	robe: string;
	pose: string;
}

export const robeToken = (id: string) => `var(--color-robe-${id})`;

export function bodyOf(cat: AppearanceCatalog | undefined, id: string | undefined): BodyDef | undefined {
	return id ? cat?.bodies.find((b) => b.id === id) : undefined;
}

/**
 * The painted outfit a look wears (decision "Bộ trang phục vẽ sẵn"): the body's outfit named by `outfitId`, else the
 * one whose accessory + robe match (chat snapshots / drafts from before outfits), else the body's default outfit —
 * the server's Resolve rule. undefined when the body is unknown.
 */
export function outfitOf(cat: AppearanceCatalog | undefined, look: Look | undefined): OutfitDef | undefined {
	const b = bodyOf(cat, look?.bodyId);
	if (!b || !look) return undefined;
	return (
		b.outfits.find((o) => o.id === look.outfitId) ??
		b.outfits.find((o) => o.hairId === look.hairId && o.robePaletteId === look.robePaletteId) ??
		b.outfits.find((o) => o.id === b.defaultOutfit)
	);
}

/** The figure of a look (undefined when the body is unknown, e.g. an older catalogue). Accessory + robe = the outfit's. */
export function figureOf(cat: AppearanceCatalog | undefined, look: Look | undefined): Figure | undefined {
	const b = bodyOf(cat, look?.bodyId);
	if (!b || !look) return undefined;
	const o = outfitOf(cat, look);
	return { sex: b.sex, path: b.path, hair: b.svgHair, beard: !!b.beard, acc: o?.hairId ?? look.hairId, robe: o?.robePaletteId ?? look.robePaletteId, pose: look.poseId };
}

const dress = (look: Look, o: OutfitDef): Look => ({ ...look, outfitId: o.id, hairId: o.hairId, robePaletteId: o.robePaletteId });

/** The body's own look (its default outfit + pose). */
export function bodyDefaults(b: BodyDef): Look {
	const base: Look = { bodyId: b.id, outfitId: b.defaultOutfit, hairId: b.defaultHair, robePaletteId: b.defaultRobe, poseId: b.defaultPose };
	const o = b.outfits.find((x) => x.id === b.defaultOutfit);
	return o ? dress(base, o) : base;
}

/** Path default (first body of the path in catalogue order — the server's rule for a character that never chose). */
export function defaultLook(cat: AppearanceCatalog, path: PathId): Look | undefined {
	const b = cat.bodies.find((x) => x.path === path);
	return b ? bodyDefaults(b) : undefined;
}

/** Canonical draft: an outfit id, with its accessory + robe (drafts saved before outfits get theirs). */
export function normalizeLook(cat: AppearanceCatalog, look: Look): Look {
	const o = outfitOf(cat, look);
	return o ? dress(look, o) : look;
}

/** Options a picker offers on `path` (GĐ-A: every shipped option is free; the server re-checks). */
export function optionsFor(cat: AppearanceCatalog, path: PathId) {
	return {
		bodies: cat.bodies.filter((b) => b.path === path && b.defaultFree),
		poses: cat.poses.filter((p) => p.path === path && p.defaultFree)
	};
}

/**
 * Pick another body in a picker: its default outfit (outfits belong to one body), and keep the pose when the new
 * body's path allows it. Pure (a draft only — the server validates on save).
 */
export function withBody(cat: AppearanceCatalog, look: Look | undefined, bodyId: string): Look | undefined {
	const b = bodyOf(cat, bodyId);
	if (!b) return look;
	const pose = look && optionsFor(cat, b.path).poses.some((p) => p.id === look.poseId) ? look.poseId : b.defaultPose;
	return { ...bodyDefaults(b), poseId: pose };
}

/** Pick another outfit of the look's body (accessory + robe follow it). */
export function withOutfit(cat: AppearanceCatalog, look: Look, outfitId: string): Look {
	const o = bodyOf(cat, look.bodyId)?.outfits.find((x) => x.id === outfitId);
	return o ? dress(look, o) : look;
}

/** Same body, outfit (its parts identify it within a body) and pose. */
export const sameLook = (a: Look | undefined, b: Look | undefined) =>
	a?.bodyId === b?.bodyId &&
	a?.hairId === b?.hairId &&
	a?.robePaletteId === b?.robePaletteId &&
	a?.poseId === b?.poseId &&
	(!a?.outfitId || !b?.outfitId || a.outfitId === b.outfitId);

// ───── raster art slots (apps/web/src/lib/assets/art/README.md) ─────
// Default outfit = the body folder; every other outfit = its own sub-folder with the same names:
// characters/<body_id>[/<outfit_id>]/avatar        256²       portrait (circle crop)
// characters/<body_id>/avatar-robe                 256²       optional grey mask of the BODY-level avatar: robe area
//                                                             tinted with the robe token (also when it stands in for
//                                                             an alternate outfit without art)
// characters/<body_id>[/<outfit_id>]/meditate[-<pose_id>]  1024²  Động Phủ / Tu Luyện (pose variant first)
// characters/<body_id>[/<outfit_id>]/body-<realm_id> | body-b<1..6>  1024×1536  Nhân Vật full body
// characters/<body_id>[/<outfit_id>]/listener      256²       Tử Tiêu Cung seat (back view)
// Chain: the outfit's folder → the body's folder (default outfit) → the parameterised SVG figure — never another
// body's art. `exact` (picker previews/cards): only the folder painting exactly this outfit, so an alternate outfit
// without art shows the SVG in its own colours instead of the default painting.

export interface SlotOpts {
	/** the body's default outfit id (its art is the body folder); unknown = the outfit folder is tried first anyway */
	defaultOutfit?: string;
	exact?: boolean;
}

/** Art folders of a look, most specific first. */
export function artDirs(look: Look, opts: SlotOpts = {}): string[] {
	const body = `characters/${look.bodyId}`;
	if (!look.outfitId || look.outfitId === opts.defaultOutfit) return [body];
	const own = `${body}/${look.outfitId}`;
	return opts.exact ? [own] : [own, body];
}

export const avatarSlots = (look: Look, opts?: SlotOpts) => artDirs(look, opts).map((d) => `${d}/avatar`);
export const maskSlot = (bodyId: string, kind: 'avatar' | 'meditate') => `characters/${bodyId}/${kind}-robe`;
/** true when `slot` is body-level art (default outfit), which an optional robe mask can recolour */
export const isBodyLevel = (bodyId: string, slot: string | undefined) => !!slot && slot.startsWith(`characters/${bodyId}/`) && slot.split('/').length === 3;
export const meditateSlots = (look: Look, opts?: SlotOpts) => artDirs(look, opts).flatMap((d) => [`${d}/meditate-${look.poseId}`, `${d}/meditate`]);
export const listenerSlots = (look: Look, opts?: SlotOpts) => artDirs(look, opts).map((d) => `${d}/listener`);
/**
 * Full-body slots per folder: a painting of this exact realm wins, then the realm's EFFECT tier and the tiers below it
 * (`body-b<k>`, decision #3: one painting per tier, not per realm), then lower realms of the path.
 */
export function bodySlots(look: Look, realmIds: string[], fxTier = 1, opts?: SlotOpts): string[] {
	const t = Math.max(1, Math.min(6, fxTier));
	return artDirs(look, opts).flatMap((d) => {
		const [own, ...lower] = realmIds.map((r) => `${d}/body-${r}`);
		const tiers = Array.from({ length: t }, (_, i) => `${d}/body-b${t - i}`);
		return [...(own ? [own] : []), ...tiers, ...lower];
	});
}

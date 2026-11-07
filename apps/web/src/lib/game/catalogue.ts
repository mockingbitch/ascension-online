import type { ItemDef, PathDef, PathId, Realm, RealmCatalogue, Stage } from './types';

/** Index of the public catalogue for name lookups in the UI. */
export class CatalogueIndex {
	readonly stages = new Map<string, Stage>();
	readonly realms = new Map<string, Realm>();
	readonly items = new Map<string, ItemDef>();
	readonly paths = new Map<PathId, PathDef>();

	constructor(cat: RealmCatalogue, items: ItemDef[]) {
		for (const p of cat.paths ?? []) this.paths.set(p.id, p);
		for (const r of cat.realms) {
			this.realms.set(r.id, r);
			for (const s of r.stages) this.stages.set(s.id, s);
		}
		for (const it of items) this.items.set(it.id, it);
	}

	realmOf(stageId: string): Realm | undefined {
		const s = this.stages.get(stageId);
		return s ? this.realms.get(s.realmId) : undefined;
	}

	/** "Luyện Khí · Luyện Khí Tầng 3" or "Trúc Cơ · Sơ Kỳ" */
	stageLabel(stageId: string): string {
		const s = this.stages.get(stageId);
		const r = this.realmOf(stageId);
		if (!s || !r) return stageId;
		return s.name.startsWith(r.name) ? s.name : `${r.name} · ${s.name}`;
	}

	/** The stage after `stageId` on the same path (next tầng, or the next realm's first stage), if any. */
	nextStage(stageId: string): Stage | undefined {
		const s = this.stages.get(stageId);
		if (!s) return undefined;
		for (const x of this.stages.values()) if (x.path === s.path && x.index === s.index + 1) return x;
		return undefined;
	}

	itemName(id: string): string {
		return this.items.get(id)?.name ?? id;
	}
}

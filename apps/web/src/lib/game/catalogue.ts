import type { ItemDef, Realm, RealmCatalogue, Stage } from './types';

/** Index of the public catalogue for name lookups in the UI. */
export class CatalogueIndex {
	readonly stages = new Map<string, Stage>();
	readonly realms = new Map<string, Realm>();
	readonly items = new Map<string, ItemDef>();

	constructor(cat: RealmCatalogue, items: ItemDef[]) {
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

	itemName(id: string): string {
		return this.items.get(id)?.name ?? id;
	}
}

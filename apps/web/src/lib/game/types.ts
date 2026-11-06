// Mirrors JSON from services/game (camelCase). Keep in sync with packages/contracts/openapi.yaml.

export type BreakthroughKind = 'auto' | 'major';
export type PathId = 'tien' | 'vu';

export interface ItemRequirement {
	itemId: string;
	quantity: number;
}

/** 1 point (điểm tu vi) = 1000 milli. Format with format.ts `formatPoints`. */
export interface Stage {
	id: string; // "<realm>.<stage>"
	realmId: string;
	path: PathId;
	tier: number;
	name: string;
	stageOrder: number;
	stageCount: number;
	index: number;
	requiredMilli: number;
	capMilli: number; // > 0 only on a realm's last stage (bình cảnh)
	cumulativeBeforeMilli: number;
	nextKind?: BreakthroughKind;
	tribulation: boolean;
	reserved?: boolean;
	breakthrough: {
		successBp: number;
		pityStepBp: number;
		cooldownSeconds: number;
		requirements: ItemRequirement[] | null;
	};
}

export type Aura =
	| 'none' | 'mist' | 'jade' | 'golden' | 'azure' | 'violet'
	| 'void' | 'crimson' | 'radiant' | 'thunder' | 'celestial'
	| 'earth' | 'aureate' | 'primal' | 'chaos' | 'saint' | 'blood';

export interface Realm {
	id: string;
	path: PathId;
	tier: number;
	name: string;
	namePlaceholder?: boolean;
	title: string;
	reserved?: boolean;
	visual: { badge: string; aura: Aura };
	stages: Stage[];
}

export interface PathDef {
	id: PathId;
	name: string;
	default: boolean;
	namePlaceholder?: boolean;
}

export interface RealmCatalogue {
	version: number;
	settings: { overflowCapBp: number; pityMaxBonusBp: number };
	paths: PathDef[];
	tribulation: { fromTier: number; guaranteeAfter: number; lossBp: number; names: Record<string, string> };
	realms: Realm[];
}

export type Bucket = 'idle' | 'activity' | 'community';

export interface BucketView {
	milli: number;
	capMilli: number;
}

/** Points credited this game day (UTC+7). */
export interface Today {
	gameDay: string;
	pointsMilli: number;
	ceilingMilli: number;
	buckets: Record<Bucket, BucketView>;
	speedItems: BucketView;
	catchUp: { active: boolean; multiplierBp: number };
	resetsAt: string;
}

/** Authoritative cultivation state (shared by the character view and settle result). */
export interface CultivationSnapshot {
	stageId: string;
	path: PathId;
	tier: number;
	cultivationMilli: number; // stored in the current stage
	requiredMilli: number;
	capMilli: number;
	bottleneckRoomMilli: number;
	grantReserveMilli: number;
	idleRateMilliPerHour: number;
	offlineCapSeconds: number;
	lastCultivatedAt: string;
	today: Today;
}

export interface CultivationStatus extends CultivationSnapshot {
	earnedMilli: number;
	rawMilli: number;
	stagesAdvanced: string[];
	hitOfflineCap: boolean;
	hitDailyCap: boolean;
	hitBottleneck: boolean;
	replayed: boolean;
}

// ───── auth / account ─────
export interface User {
	id: string;
	email: string;
	displayName: string;
	avatarUrl: string;
	role: 'player' | 'admin';
}

export interface SessionResponse {
	accessToken: string;
	expiresIn: number;
	user: User;
}

// ───── character ─────
export interface CharacterView extends CultivationSnapshot {
	id: string;
	name: string;
	createdAt: string;
	realmId: string;
	failStreak: number;
	cooldownUntil: string | null;
	stageReachedAt: string;
	offlineBonusSeconds: number;
	bonusUsedToday: boolean;
}

// ───── breakthrough ─────
export interface Missing {
	kind: 'cultivation' | 'item' | 'cooldown' | 'final_stage';
	itemId?: string;
	have: number; // cultivation: milli since the start of the realm
	need: number;
	readyAt?: string;
}

export interface OddsPart {
	source: 'base' | 'pity' | 'surplus' | 'items' | 'protector';
	bp: number;
}

export interface TribulationPreview {
	name: string;
	guaranteed: boolean;
	failStreak: number;
	guaranteeAfter: number;
	lossBp: number;
	lossMilli: number; // loss if attempted now — show this
	maxLossMilli: number; // guard bound — send this as acceptLossMilli
	cooldownSeconds: number;
}

export interface BreakthroughPreview {
	from: string;
	to?: string;
	kind?: BreakthroughKind;
	ready: boolean;
	missing: Missing[];
	consumes: ItemRequirement[];
	settlementIncluded: boolean;
	projectedCultivationMilli: number;
	successBp: number;
	odds: OddsPart[];
	cooldownSeconds: number;
	tribulation?: TribulationPreview;
}

export interface BreakthroughResult {
	success: boolean;
	kind: BreakthroughKind;
	from: string;
	to: string;
	successBp: number;
	consumed: ItemRequirement[] | null;
	tribulation: boolean;
	guaranteed: boolean;
	lossMilli: number;
	stageId: string;
	cultivationMilli: number;
	stagesAdvanced: string[];
	failStreak: number;
	cooldownUntil: string | null;
	replayed: boolean;
}

// ───── items ─────
export type Rarity = 'pham' | 'linh' | 'huyen' | 'dia' | 'thien' | 'tien';
export interface ItemDef {
	id: string;
	name: string;
	category: string;
	rarity: Rarity;
	tradable: boolean;
	description: string;
	offlineCapBonusHours?: number;
}

// ───── VIP ─────
export interface VipBenefits {
	tier: number;
	name: string;
	offlineCapHours: number;
	dailyGift: number;
	alchemySlots: number;
	cosmetics: string[];
	flags: string[];
}
export interface VipTier {
	tier: number;
	pointsRequired: number;
	benefits: VipBenefits;
}
export interface VipStatus {
	points: number;
	benefits: VipBenefits;
	nextTier?: number;
	pointsToNext?: number;
	giftClaimedToday: boolean;
}

export interface GiftResult {
	tier: number;
	amount: number;
	currency: string;
	balance: number;
	nextResetAt: string;
}

// ───── inventory / tasks ─────
export interface Inventory {
	currencies: Record<string, number>;
	items: { itemId: string; quantity: number }[];
}

export interface TaskReward {
	itemId?: string;
	quantity?: number;
	currency?: string;
	amount?: number;
}

export interface DailyTask {
	id: string;
	name: string;
	description: string;
	kind: 'checkin' | 'settle' | 'breakthrough_attempt';
	target: number;
	rewards: TaskReward[];
	progress: number;
	complete: boolean;
	claimed: boolean;
}

export interface DailyBoard {
	gameDay: string;
	resetsAt: string;
	tasks: DailyTask[];
}

export interface BonusStatus {
	itemId: string;
	remaining: number;
	offlineBonusSeconds: number;
	replayed: boolean;
}

// ───── realtime ─────
export interface Announcement {
	v: number;
	type: 'announcement.major_breakthrough';
	id: string;
	ts: string;
	data: { characterId: string; characterName: string; realmId: string; stageId: string; text: string };
}

// ───── leaderboard ─────
export interface LeaderboardEntry {
	rank: number;
	characterId: string;
	name: string;
	stageId: string;
	score: number;
	stageReachedAt: string;
}
export interface LeaderboardPage {
	entries: LeaderboardEntry[];
	total: number;
	offset: number;
}

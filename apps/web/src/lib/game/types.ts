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
	| 'earth' | 'aureate' | 'primal' | 'chaos' | 'saint' | 'blood'
	// Vu path: one token per effect tier (decisions 2026-11-09 #10)
	| 'vu-ember' | 'vu-blood' | 'vu-crimson' | 'vu-storm' | 'vu-totem' | 'vu-ancestral';

export interface Realm {
	id: string;
	path: PathId;
	tier: number;
	name: string;
	namePlaceholder?: boolean;
	title: string;
	reserved?: boolean;
	/** fxTier 1–6: the effect tier of the realm's tier (realms.yaml tiers[].visual.fx_tier), shared by both paths */
	visual: { badge: string; aura: Aura; fxTier: number };
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
	/** null = no faction yet (the game guard sends the player to the faction step) */
	factionId: string | null;
	/** equipped cosmetics (ids only; resolved with GET /cosmetics/catalog) */
	appearance: Appearance;
	/** only on an onboarding replay (same Idempotency-Key) */
	replayed?: boolean;
}

// ───── breakthrough ─────
export interface Missing {
	kind: 'cultivation' | 'item' | 'trial' | 'cooldown' | 'final_stage';
	itemId?: string;
	/** kind 'trial': the trials.yaml kind whose valid pass is needed (e.g. tam_ma) */
	trialKind?: string;
	have: number; // cultivation: milli since the start of the realm
	need: number;
	readyAt?: string;
}

/**
 * One disclosed component of the success chance. For a non-guaranteed tribulation the parts sum to successBp: bonus parts
 * uncapped, then bonus_cap (≤ 0, total-bonus cap) and max_cap (≤ 0, final cap); a guaranteed attempt adds guarantee.
 */
export interface OddsPart {
	source: 'base' | 'pity' | 'surplus' | 'items' | 'protector' | 'bonus_cap' | 'max_cap' | 'guarantee';
	bp: number;
	/** items: equipped pháp bảo id; protector: counted members */
	detail?: string;
}

/** Decision-F caps and bonus sources of a tribulation preview (Phase 3.4). */
export interface OddsCaps {
	surplusMaxBp: number;
	itemMaxBp: number;
	protectorPerMemberBp: number;
	protectorMaxBp: number;
	totalBonusMaxBp: number;
	maxSuccessBp: number;
	artifactId: string;
	artifactBonusBp: number;
	protectorMembers: number;
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
	/** the mini-game gate of this breakthrough (pass kept through failures, consumed by the success) */
	trial?: TrialGate;
	/** tribulations: the decision-F caps + bonus sources (pháp bảo, hộ pháp) */
	caps?: OddsCaps;
}

export interface TrialGate {
	kind: string;
	name: string;
	passed: boolean;
	validUntil: string | null;
}

export interface BreakthroughResult {
	success: boolean;
	kind: BreakthroughKind;
	from: string;
	to: string;
	successBp: number;
	consumed: ItemRequirement[] | null;
	tribulation: boolean;
	/** trial kind whose pass this successful breakthrough consumed */
	trialConsumed?: string;
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
	/** Speed item: points credited per use (source item, activity bucket, speed cap). */
	instantPointsMilli?: number;
	maxUsePerGameDay?: number;
	uniquePerCharacter?: boolean;
	/** Pháp bảo (category artifact): tribulation odds bonus while equipped. Bí cảnh drop only, never sold or traded. */
	tribulationBonusBp?: number;
}

// ───── VIP (player-facing name: Tiên Duyên; points: điểm duyên) ─────
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
/** One in-game source of điểm duyên (vip.yaml earn.activities). live = false: feature not released yet. */
export interface VipActivity {
	id: string;
	source: string;
	name: string;
	points: number;
	dailyMaxTimes: number;
	live: boolean;
	minElapsedSeconds?: number;
}
export interface VipActivityToday extends VipActivity {
	timesToday: number;
	pointsToday: number;
}
export interface VipTiersResponse {
	tiers: VipTier[];
	displayName: string;
	pointName: string;
	pointsPer1000Vnd: number;
	earnDailyCap: number;
	activities: VipActivity[];
}
export interface VipStatus {
	displayName: string;
	pointName: string;
	points: number;
	benefits: VipBenefits;
	nextTier?: number;
	pointsToNext?: number;
	giftClaimedToday: boolean;
	/** In-game điểm duyên earned this game day (top-up and admin grants excluded). */
	earnedToday: number;
	earnDailyCap: number;
	activities: VipActivityToday[];
	resetsAt: string;
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

export interface StreakReward {
	minStreak: number;
	pointsMilli: number;
}

/** Incense streak: current = still alive today (0 = broken); nextPointsMilli = what the next offering pays. */
export interface StreakView {
	current: number;
	best: number;
	offeredToday: boolean;
	nextPointsMilli: number;
}

export interface DailyTask {
	id: string;
	name: string;
	description: string;
	kind: 'checkin' | 'settle' | 'breakthrough_attempt' | 'lecture_attend' | 'incense';
	target: number;
	rewards: TaskReward[];
	streakRewards?: StreakReward[];
	streak?: StreakView;
	progress: number;
	complete: boolean;
	claimed: boolean;
	/** Điểm duyên paid by the claim (claim response only; absent when capped for the day). */
	vipPoints?: number;
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

/** POST /items/{id}/use for a speed item (Tụ Linh Đan). */
export interface SpeedItemResult {
	itemId: string;
	remaining: number;
	usesToday: number;
	maxUsesPerDay: number;
	rawMilli: number;
	earnedMilli: number;
	stagesAdvanced: string[];
	cultivation: CultivationSnapshot;
	replayed: boolean;
}

// ───── shop (linh thạch sink) ─────
export type ShopLockedReason = 'tier' | 'owned' | 'sold_out' | 'stones';
export interface ShopListing {
	id: string;
	itemId: string;
	lotSize: number;
	price: number;
	/** absent = no daily limit */
	dailyLimitLots?: number;
	minTier?: number;
	name: string;
	category: string;
	rarity: Rarity;
	boughtTodayLots: number;
	/** -1 = no daily limit */
	remainingTodayLots: number;
	owned: number;
	available: boolean;
	lockedReason?: ShopLockedReason;
}
export interface ShopView {
	currency: string;
	balance: number;
	maxLotsPerOrder: number;
	tier: number;
	gameDay: string;
	resetsAt: string;
	listings: ShopListing[];
}
export interface BuyResult {
	purchaseId: number;
	listingId: string;
	lots: number;
	totalPrice: number;
	item: { itemId: string; quantity: number };
	replayed: boolean;
	shop: ShopView;
}

// ───── daily rituals ─────
export interface IncenseResult {
	streak: StreakView;
	rawMilli: number;
	earnedMilli: number;
	vipPoints: number;
	stagesAdvanced: string[];
	task: DailyTask;
	cultivation: CultivationSnapshot;
	replayed: boolean;
}

/** Gieo quẻ outcome; weightBp is the disclosed chance (Σ = 10000). */
export interface FortuneOutcome {
	id: string;
	name: string;
	weightBp: number;
	rateBonusBp: number;
	verse: string;
}
export interface FortuneView {
	name: string;
	outcomes: FortuneOutcome[];
	today: { outcome: FortuneOutcome; rollBp: number; drawnAt: string } | null;
	buff: { bonusBp: number; startsAt: string; endsAt: string } | null;
	gameDay: string;
	resetsAt: string;
	replayed?: boolean;
	cultivation?: CultivationSnapshot;
}

// ───── realtime ─────
interface Envelope<T extends string, D> {
	v: number;
	type: T;
	id: string;
	ts: string;
	data: D;
}
/** Feed items: every one carries a server-rendered `data.text` (packages/contracts/ws-messages.md). */
/** A world announcement (GĐ-D, decisions 2026-11-09 #7 #8 #12): GET /announcements/recent and the WS
 *  `announcement.*` data. Public fields only — the đạo hiệu names the player. */
export interface WorldAnnouncement {
	/** history row id (= WS envelope id) */
	id: number;
	kind: 'realm' | 'tribulation' | 'first_to_realm' | 'rank_top10' | 'digest';
	/** effect tier of the realm (0 = none, e.g. rank news) */
	tier: number;
	characterId?: string;
	characterName: string;
	realmId?: string;
	stageId?: string;
	path?: PathId;
	tribulation: boolean;
	rank?: number;
	/** digest: how many others */
	count?: number;
	text: string;
	createdAt: string;
}
export type NewsType = 'announcement.major_breakthrough' | 'announcement.first_to_realm' | 'announcement.rank_top10' | 'announcement.digest';

export type Announcement =
	| Envelope<NewsType, WorldAnnouncement>
	| Envelope<'world.milestone_reached', WorldMilestoneReachedData>
	| Envelope<'world.event_ended', WorldEventEndedData>
	| Envelope<'faction.announcement', FactionAnnouncementData>;

// ───── world (story clock, GET /world) ─────
export type WorldReason = 'threshold' | 'max_time' | 'admin';
export interface WorldEra {
	id: string;
	name: string;
}
export interface WorldReached {
	id: string;
	name: string;
	reachedAt: string;
	serverDay: number;
	reason: WorldReason;
}
export interface WorldNextMilestone {
	id: string;
	name: string;
	/** effective target day (admin override applied) */
	targetDay: number;
	thresholdPerActiveMilli: number;
	progressBp: number;
	/** 70% floor / 150% ceiling of targetDay; null before launch */
	earliestAt: string | null;
	latestAt: string | null;
	estimatedAt: string | null;
}
export interface WorldActiveEvent {
	id: string;
	milestoneId: string;
	name: string;
	startedAt: string;
	endsAt: string;
}
export interface WorldPendingEffect {
	kind: 'set_era' | 'start_event' | 'split_faction' | 'migrate_faction';
	milestoneId: string;
	dueAt: string;
	deadlineAt?: string;
	from?: string;
	into?: string[];
	to?: string;
}
export interface WorldTimelineEntry {
	id: string;
	name: string;
	targetDay: number;
	/** ranged event length in days, 0 = none */
	eventDays: number;
	reached: boolean;
	reachedAt: string | null;
	reachedDay: number | null;
}
export interface WorldView {
	started: boolean;
	startedAt: string | null;
	/** -1 before launch */
	serverDay: number;
	paused: boolean;
	era: WorldEra | null;
	progress: {
		perActiveMilli: number;
		closedThroughDay?: string;
		todayProvisionalMilli: number;
		yesterdayDau: number;
		referencePerDayMilli: number;
	};
	lastMilestone: WorldReached | null;
	nextMilestone: WorldNextMilestone | null;
	activeEvents: WorldActiveEvent[];
	pendingEffects: WorldPendingEffect[];
	timeline: WorldTimelineEntry[];
}
export interface WorldMilestoneReachedData {
	milestoneId: string;
	name: string;
	seq: number;
	reason: WorldReason;
	serverDay: number;
	eraId: string;
	eraName: string;
	text: string;
}
export interface WorldEventEndedData {
	eventId: string;
	milestoneId: string;
	serverDay: number;
	eraId: string;
	eraName: string;
	text: string;
}

// ───── reports / moderation ─────
export type ReportReason = 'offensive_name' | 'harassment' | 'trade_abuse';
export type ReportStatus = 'open' | 'dismissed' | 'actioned';
export interface Report {
	id: string;
	targetCharacterId: string;
	/** name at report time */
	targetName: string;
	reason: ReportReason;
	note: string;
	status: ReportStatus;
	createdAt: string;
	chatMessageId: number | null;
	/** trade_abuse reports: the offer reported */
	tradeOfferId?: string | null;
}
export interface AdminReport extends Report {
	reporterUserId: string;
	reporterName: string;
	targetUserId: string;
	targetCurrentName: string;
	openAgainstTarget: number;
	resolutionNote: string;
	resolvedAt: string | null;
	/** original chat text at report time */
	evidence: string;
}
export interface AdminReportPage {
	reports: AdminReport[];
	limit: number;
	offset: number;
}
export interface ReportResolution {
	report: Report;
	resolutionNote: string;
	renamedFrom?: string;
	renamedTo?: string;
	alsoResolved: string[];
}

// ───── chat ─────
export interface ChatChannel {
	id: string;
	name: string;
	/** stage needed to post (display hint; the server enforces) */
	minStage: string;
}
export interface ChatMessage {
	id: number;
	channel: string;
	characterId: string;
	/** sender name at send time */
	name: string;
	stageId: string;
	/** masked text */
	text: string;
	sentAt: string;
	/** chat_aura cosmetic the sender wore when posting (display only) */
	nameFx?: string;
	/** sender look at send time (display only; absent on old messages) */
	look?: Look;
}
export interface ChatHistory {
	channel: ChatChannel;
	messages: ChatMessage[];
	maxChars: number;
}
/** WS envelopes other than announcements */
export type ChatEvent =
	| { v: number; type: 'chat.message'; id: string; ts: string; data: ChatMessage }
	| { v: number; type: 'chat.hidden'; id: string; ts: string; data: { id: number; channel: string } };

// ───── Tử Tiêu Cung lectures ─────
export interface LectureLine {
	text: string;
	seconds: number;
}
export interface LectureScript {
	id: string;
	order: number;
	title: string;
	theme: string;
	lines: LectureLine[];
	question: string;
	takeaway: string;
	/** Σ line seconds */
	seconds: number;
}
export interface LectureScripts {
	lectures: LectureScript[];
	cycleSeconds: number;
	cycleEpoch: string;
}
export interface Listener {
	characterId: string;
	name: string;
	stageId: string;
	/** -1 = listening outside the hall */
	seat: number;
	/** listener look (display only) */
	look?: Look;
}
export interface InsightSlot {
	no: number;
	opensAt: string;
	closesAt: string;
}
export interface LectureToday {
	gameDay: string;
	milli: number;
	capMilli: number;
}
export interface LectureMe {
	inHall: boolean;
	seat: number;
	/** unpaid listening (server clock), paid at the next ngộ đạo */
	accruedSeconds: number;
}
export interface LectureState {
	serverNow: string;
	cycleEpoch: string;
	cycleSeconds: number;
	seats: number;
	listeners: number;
	seated: Listener[];
	chatChannel: string;
	heartbeatSeconds: number;
	rateMilliPerHour: number;
	maxSecondsPerInsight: number;
	insight: {
		current: InsightSlot;
		open: boolean;
		claimed: boolean;
		next: InsightSlot;
		everySeconds: number;
	};
	today: LectureToday;
	me: LectureMe;
}
export interface InsightResult {
	cultivation: CultivationSnapshot;
	presenceSeconds: number;
	rawMilli: number;
	earnedMilli: number;
	vipPoints: number;
	stagesAdvanced: string[];
	lectureId: string;
	takeaway: string;
	today: LectureToday;
	replayed: boolean;
}
export interface NoteEntry {
	lectureId: string;
	title: string;
	theme: string;
	takeaway: string;
	firstAt: string;
	times: number;
}
export type LectureEvent =
	| { v: number; type: 'lecture.enter'; id: string; ts: string; data: Listener }
	| {
			v: number;
			type: 'lecture.leave';
			id: string;
			ts: string;
			data: Listener;
	  };

// ───── leaderboard ─────
export interface LeaderboardEntry {
	rank: number;
	characterId: string;
	name: string;
	stageId: string;
	score: number;
	stageReachedAt: string;
	/** equipped cosmetics (display only, never part of the score) */
	avatarFrameId?: string;
	titleId?: string;
	/** chosen look resolved for the row's path (display only) */
	look?: Look;
}
export interface LeaderboardPage {
	entries: LeaderboardEntry[];
	total: number;
	offset: number;
}

// ───── factions (GET /factions, /factions/me; data model §6) ─────
/** Catalogue entry + live numbers. `color` is a design token suffix: var(--color-<color>). */
export interface FactionView {
	id: string;
	name: string;
	namePlaceholder: boolean;
	allowedPaths: PathId[];
	/** absent = never open yet (canon not decided) */
	openFromEra?: string;
	closeFromEra?: string;
	color: string;
	motto?: string;
	pitch?: string;
	/** joinable in the current era */
	open: boolean;
	members: number;
	/** credited tu vi of members in the current era (milli) */
	meritMilli: number;
	/** 1-based among open or populated factions; 0 = unranked */
	meritRank: number;
}
export interface FactionList {
	factions: FactionView[];
	switchCooldownSeconds: number;
	eraId: string;
	/** first game day counted toward merit (null before launch) */
	meritSince: string | null;
	/** false → counts from Postgres, merit unknown */
	rankingAvailable: boolean;
}
export interface FactionEffectNotice {
	kind: 'split_faction' | 'migrate_faction';
	from: string;
	into?: string[];
	to?: string;
	dueAt: string;
	deadlineAt?: string;
}
export type FactionChangeReason = 'join' | 'switch' | 'split_choice' | 'split_default' | 'auto_migrate' | 'reincarnation' | 'admin';
export interface FactionHistoryEntry {
	from: string | null;
	to: string;
	fromPath?: PathId;
	toPath?: PathId;
	reason: FactionChangeReason;
	createdAt: string;
}
export interface FactionMembership {
	/** null = no faction yet */
	factionId: string | null;
	joinedAt: string | null;
	/** null = a switch is allowed now */
	canSwitchAt: string | null;
	switchCooldownSeconds: number;
	locked: boolean;
	lockReason?: 'fengshen' | 'admin' | 'event';
	lockedUntil?: string | null;
	/** branches of a pending split; empty = nothing to choose */
	mustChoose: string[];
	chooseDeadline: string | null;
	path: PathId;
	stageId: string;
	upcoming: FactionEffectNotice | null;
	history: FactionHistoryEntry[];
}
export interface FactionChangeResult {
	membership: FactionMembership;
	replayed: boolean;
}
export interface FactionMemberEntry {
	rank: number;
	characterId: string;
	name: string;
	stageId: string;
	avatarFrameId?: string;
	titleId?: string;
	look?: Look;
}
export interface FactionMembersPage {
	factionId: string;
	entries: FactionMemberEntry[];
	total: number;
	nextCursor: string | null;
}
/** WS faction.announcement (split/migrate notices from the world clock). */
export interface FactionAnnouncementData {
	key: 'faction.split_notice' | 'faction.split_done' | 'faction.split_final' | 'faction.migrate_notice' | 'faction.migrate_done';
	kind: 'split_faction' | 'migrate_faction';
	from: string;
	targets: string[];
	dueAt: string;
	deadlineAt?: string | null;
	text: string;
}

// ───── trials (mini games, Phase 3) ─────
/** Tâm Ma rules snapshot (trials.yaml params; the run's copy is what the server replays). */
export interface TamMaParams {
	rulesVersion: number;
	tickMs: number;
	width: number;
	height: number;
	timeLimitTicks: number;
	passLines: number;
	gravity: { startTicks: number; minTicks: number; stepEveryTicks: number };
	riseEveryTicks: number;
	calm: { chargePerLine: number; chargeMax: number; durationTicks: number };
	lineScores: number[];
	maxInputs: number;
	pieces: { id: string; name: string; weight: number; cells: [number, number][] }[];
	/** rules v2 — human plausibility: ≤ maxInputsPerWindow inputs in any inputWindowTicks window (else rejected) */
	inputWindowTicks: number;
	maxInputsPerWindow: number;
	/** a drop sooner than this after the previous lock is ignored (game rule) */
	minLockGapTicks: number;
	/** the pass needs passLines AND tick ≥ minPassTicks */
	minPassTicks: number;	/** kind-level snapshot: the server-clock floor (bp of game time) this run is judged with */
	minElapsedBp?: number;
}

/** Hóa Phàm rules snapshot (trials.yaml params; docs/design/hoa-pham-luat-choi.md). Grid units; motion in 1/32 units. */
export interface HoaPhamParams {
	rulesVersion: number;
	tickMs: number;
	width: number;
	height: number;
	timeLimitTicks: number;
	passScore: number;
	minPassTicks: number;
	/** tâm đăng: cutting this many chân tâm lanterns fails */
	lanterns: number;
	gravitySub: number;
	knotMotion: { vySub: { min: number; max: number }; vxMaxSub: number };
	lanternMotion: { vySub: { min: number; max: number }; vxMaxSub: number };
	spawn: { startTicks: number; minTicks: number; stepEveryTicks: number; waveMax: number; waveGrowEveryTicks: number; margin: number };
	minCutStep: number;
	combo: { stepBonus: number; maxBonus: number };
	calm: { chargePerCut: number; chargeMax: number; durationTicks: number };
	maxStrokes: number;
	objects: { id: string; name: string; role: 'knot' | 'lantern'; weight: number; radius: number; points: number; hp: number }[];
	/** human plausibility (also game rules the recorder applies): brush ≤ maxStep units/tick … */
	maxStep: number;
	/** … ≤ maxWindowTravel units and ≤ maxStrokesPerWindow stroke starts in any inputWindowTicks window … */
	inputWindowTicks: number;
	maxWindowTravel: number;
	maxStrokesPerWindow: number;
	/** … and ≥ minLiftTicks unpressed ticks between strokes */
	minLiftTicks: number;	/** kind-level snapshot: the server-clock floor (bp of game time) this run is judged with */
	minElapsedBp?: number;
}

/** Params of a trial kind (by engine: tam_ma → TamMaParams, hoa_pham → HoaPhamParams). */
export type TrialParams = TamMaParams | HoaPhamParams;

export type TrialState = 'available' | 'in_progress' | 'cooldown' | 'passed' | 'locked';

export interface TrialKindStatus {
	kind: string;
	name: string;
	summary: string;
	state: TrialState;
	eligible: boolean;
	gateTier: number;
	fromRealmId: string;
	toRealmId: string;
	opensAt: 'last_stage' | 'realm';
	cooldownUntil: string | null;
	pass: { runId: string; validUntil: string } | null;
	openRun: { runId: string; expiresAt: string } | null;
	loseCooldownSeconds: number;
	passValidSeconds: number;
	expireAfterSeconds: number;
	params: TrialParams;
}

export interface TrialStatus {
	serverNow: string;
	kinds: TrialKindStatus[];
}

export interface TrialRunStart {
	runId: string;
	kind: string;
	seed: number;
	issuedAt: string;
	expiresAt: string;
	serverNow: string;
	params: TrialParams;
	resumed: boolean;
	status: TrialKindStatus;
}

/** What the client sends: its input log + the end state its simulator reached (checked against the server replay). */
export interface TrialClaim {
	endTick: number;
	/** tâm ma: rows cleared; hóa phàm: knots severed */
	lines: number;
	score: number;
	stateHash: number;
	/** tâm ma: [tick, action] */
	inputs: [number, number][];
	/** hóa phàm: [start, x0, y0, dx1, dy1, …] per stroke (grid units) */
	strokes?: number[][];
}

export type TrialVerdict = 'pass' | 'fail' | 'rejected' | 'expired';

export interface TrialOutcome {
	won: boolean;
	reason: string;
	endTick: number;
	gameMillis: number;
	lines: number;
	score: number;
	pieces: number;
	stateHash: number;
	/** cadence telemetry (audit) */
	inputs: number;
	inputsPerSecMilli: number;
	maxWindowInputs: number;
	minLockGapTicks: number;
	claimedEndTick: number;
	/** hóa phàm telemetry (lines = knots severed, pieces = objects spawned, inputs = pointer samples) */
	strokes?: number;
	maxStep?: number;
	maxWindowTravel?: number;
	maxWindowStrokes?: number;
	lanternsCut?: number;
	escaped?: number;
	bestCombo?: number;
	/** server ms from issue to submit; slowSubmit = beyond game time + slack (review flag only) */
	elapsedMillis?: number;
	slowSubmit?: boolean;
}

export interface TrialSubmitResult {
	runId: string;
	kind: string;
	verdict: TrialVerdict;
	/** cleared | topped_out | lanterns_out | time_up | gave_up | late | abandoned | invalid_log | implausible_score | implausible_input | too_fast | mismatch */
	reason: string;
	/** the server replay — only for pass/fail verdicts */
	outcome: TrialOutcome | null;
	passValidUntil: string | null;
	cooldownUntil: string | null;
	vipPoints: number;
	replayed: boolean;
	status: TrialKindStatus;
}

// ───── cosmetics (GET /cosmetics/catalog, /cosmetics; decisions 2026-11-07 "Linh thạch sinks") ─────
// No gameplay effect: titles, portrait frames, chat name effects and cave decorations only change how a character looks.

export type CosmeticKind = 'title' | 'avatar_frame' | 'chat_aura' | 'cave_decoration';
export type CosmeticSource = 'shop' | 'tien_duyen';

/** Equipped look. Ids are "<kind>:<slug>"; empty/absent = nothing in that slot. */
export interface Appearance {
	titleId?: string;
	avatarFrameId?: string;
	chatAuraId?: string;
	caveDecorationIds: string[];
	/** body / accessory / robe / pose. Response: the look shown for the current path. Equip: optional (absent = keep). */
	look?: Look;
}

// ───── appearances (content/catalog/appearances.yaml; decisions 2026-11-09 "Ngoại hình…") ─────
// Display only: a look never changes tu vi, odds, rank or điểm duyên.

/**
 * A character's look: body + painted outfit + pose (decision "Bộ trang phục vẽ sẵn"). Served resolved: complete and
 * valid for the path. hairId / robePaletteId are the outfit's parts (derived by the server; requests may carry them,
 * the server takes them from the outfit).
 */
export interface Look {
	bodyId: string;
	/** one of the body's outfits; absent only on chat snapshots older than outfits (derive with `outfitOf`) */
	outfitId?: string;
	hairId: string;
	/** web token --color-robe-<id> */
	robePaletteId: string;
	poseId: string;
}
export type BodySex = 'nam' | 'nu';
/** SVG fallback hairstyles (CultivatorFigure); the server validates the same list. */
export type SvgHair =
	| 'bui_cao'
	| 'bui_nua'
	| 'duoi_ngua'
	| 'van_cao'
	| 'tet_lech'
	| 'xoa_buoc_thap'
	| 'bim_nhieu'
	| 'chom'
	| 'tet_duoi'
	| 'ngan_khan';
export interface BodyDef {
	id: string;
	path: PathId;
	sex: BodySex;
	name: string;
	temperament: string;
	traits: string;
	svgHair: SvgHair;
	beard?: boolean;
	/** parts of defaultOutfit */
	defaultHair: string;
	defaultRobe: string;
	defaultOutfit: string;
	defaultPose: string;
	/** same-sex body on the other path (shown after a path change) */
	pair: string;
	defaultFree: boolean;
	/** exactly 3 painted outfits; art in characters/<body>[/<outfit>]/… (the default outfit = the body folder) */
	outfits: OutfitDef[];
}
/** A painted outfit: robe colour + hair accessory baked into the body's art. */
export interface OutfitDef {
	id: string;
	name: string;
	robePaletteId: string;
	hairId: string;
}
export interface LookOption {
	id: string;
	name: string;
	paths: PathId[];
	defaultFree: boolean;
}
export interface PoseDef {
	id: string;
	name: string;
	path: PathId;
	defaultFree: boolean;
}
export interface AppearanceCatalog {
	settings: { bodyChangeCooldownSeconds: number };
	bodies: BodyDef[];
	hairAccessories: LookOption[];
	robePalettes: LookOption[];
	poses: PoseDef[];
}
export interface CosmeticDef {
	id: string;
	slug: string;
	kind: CosmeticKind;
	series: string;
	name: string;
	rarity: Rarity;
	source: CosmeticSource;
	/** spirit stones (shop only) */
	price?: number;
	/** Tiên Duyên level needed to buy (shop) */
	minTier?: number;
	/** Tiên Duyên level that grants it (tien_duyen) */
	perkTier?: number;
	description?: string;
	season?: string;
	fromEra?: string;
	untilEra?: string;
}
export interface CosmeticSeries {
	id: string;
	kind: CosmeticKind;
	name: string;
	source: CosmeticSource;
	season?: string;
	fromEra?: string;
	untilEra?: string;
	items: string[];
}
export interface CosmeticSeason {
	id: string;
	name: string;
	months: number[];
}
export interface CosmeticCatalog {
	settings: { currency: 'spirit_stone'; caveSlots: number };
	seasons: CosmeticSeason[];
	series: CosmeticSeries[];
	cosmetics: CosmeticDef[];
	/** body / accessory / robe / pose catalogue (absent on an older server) */
	appearances?: AppearanceCatalog;
}
export interface CosmeticsView {
	balance: number;
	tier: number;
	season: string;
	/** '' before launch (first era applies) */
	eraId: string;
	owned: string[];
	/** shop cosmetics inside their season/era window right now */
	available: string[];
	equipped: Appearance;
	/** when the next body change is allowed (null = now) */
	bodyRetryAt?: string | null;
}
export interface CosmeticBuyResult {
	cosmeticId: string;
	price: number;
	replayed: boolean;
	state: CosmeticsView;
}
export interface CosmeticEquipResult {
	replayed: boolean;
	state: CosmeticsView;
}

// ───── faction works (GET /factions/works, POST /factions/donate) ─────
// Donations build purely visual faction decorations; they add no merit, rank, điểm duyên or tu vi.

export interface FactionWorkStage {
	at: number;
	decoration: { slug: string; name: string };
}
export interface FactionWork {
	id: string;
	name: string;
	description: string;
	goal: number;
	/** stones donated before this work starts (Σ earlier goals) */
	start: number;
	stages: FactionWorkStage[];
}
export interface FactionWorksProgress {
	total: number;
	/** current work; = works.length when everything is built */
	workIndex: number;
	workDonated: number;
	/** decoration slugs unlocked, build order */
	unlocked: string[];
}
export interface FactionDonor {
	characterId: string;
	name: string;
	total: number;
}
export interface FactionWorksView {
	settings: { currency: 'spirit_stone'; dailyCapPerCharacter: number; minAmount: number; donorBoardSize: number };
	works: FactionWork[];
	factions: { factionId: string; donatedTotal: number }[];
	balance: number;
	todayDonated: number;
	gameDay: string;
	resetsAt: string;
	/** null = not in a faction (cannot donate) */
	mine: { factionId: string; progress: FactionWorksProgress; myTotal: number; donors: FactionDonor[] } | null;
}
export interface FactionDonateResult {
	donationId: number;
	amount: number;
	replayed: boolean;
	state: FactionWorksView;
}

// ───── bí cảnh (GET /secret-realms, start/submit; Phase 3.4, docs/design/bi-canh-luat-choi.md) ─────

export interface BiCanhEnemy {
	id: string;
	name: string;
	weight: number;
	fromTick: number;
	hp: number;
	speedSub: number;
	radius: number;
	points: number;
	atTicks?: number[];
}

/** Rules snapshot of a bí cảnh run (rules v1). Positions in arena units; *Sub in 1/16 unit per tick. */
export interface BiCanhParams {
	rulesVersion: number;
	tickMs: number;
	timeLimitTicks: number;
	arena: { width: number; height: number };
	player: { radius: number; speedSub: number; hp: number; invulnTicks: number };
	sword: { range: number; speedSub: number; radius: number; lifeTicks: number; damage: number };
	orbit: { radius: number; bladeRadius: number; step: number; damage: number; hitCooldownTicks: number };
	levels: { qi: number; fireEvery: number; orbit: number }[];
	spawn: { startTicks: number; minTicks: number; stepEveryTicks: number; waveMax: number; waveGrowEveryTicks: number; maxEnemies: number; safeRadius: number };
	enemies: BiCanhEnemy[];
	orbs: { pickupRadius: number; magnetRadius: number; magnetSpeedSub: number; lifeTicks: number; max: number };
	maxSwords: number;
	/** human cadence: ≤ maxChangesPerWindow direction changes in any inputWindowTicks */
	inputWindowTicks: number;
	maxChangesPerWindow: number;
	maxRuns: number;
	/** run-level snapshot */
	minElapsedBp?: number;
	minRewardTicks?: number;
}

/** A reward band: score AND linh khí picked up AND spirits defeated (every band needs real play). */
export interface SecretRealmBand {
	id: string;
	name: string;
	minScore: number;
	minQi: number;
	minKills: number;
}

export interface SecretRealmBandLoot {
	spiritStone: { min: number; max: number };
	items: { itemId: string; min: number; max: number }[];
	artifactChanceBp: number;
}

export interface SecretRealmLoot {
	band: string;
	spiritStone: number;
	items: { itemId: string; quantity: number }[];
	/** pháp bảo granted */
	artifact?: string;
	/** rolled pháp bảo already owned → duplicate stones */
	duplicateOf?: string;
	artifactChanceBp: number;
	rolledSpiritStone: number;
	rolledItems: { itemId: string; quantity: number }[];
	rolledArtifact?: string;
	/** a daily cap reduced something */
	clipped: boolean;
	artifactCapped?: boolean;
}

export interface SecretRealmStatus {
	realmId: string;
	name: string;
	summary: string;
	unlocked: boolean;
	minTier: number;
	runsPerDay: number;
	runsLeft: number;
	expireAfterSeconds: number;
	minRewardTicks: number;
	bands: SecretRealmBand[];
	loot: Record<string, SecretRealmBandLoot>;
	survived: { spiritStone: number; artifactChanceBp: number };
	artifacts: { itemId: string; weight: number; duplicateStones: number }[];
	dailyCaps: { spiritStone: number; materials: number; artifacts: number };
	today: { gameDay: string; runs: number; spiritStone: number; materials: number; artifacts: number };
	resetAt: string;
	openRun: { runId: string; expiresAt: string } | null;
	params: BiCanhParams;
}

export interface SecretRealmRecent {
	runId: string;
	realmId: string;
	issuedAt: string;
	verdict: TrialVerdict;
	reason: string;
	band: string;
	score: number;
	loot: SecretRealmLoot | null;
}

export interface SecretRealmView {
	serverNow: string;
	realms: SecretRealmStatus[];
	recent: SecretRealmRecent[];
}

export interface SecretRealmStart {
	runId: string;
	realmId: string;
	seed: number;
	issuedAt: string;
	expiresAt: string;
	serverNow: string;
	params: BiCanhParams;
	resumed: boolean;
	status: SecretRealmStatus;
}

/** What the client sends: its RLE movement log + the end state its simulator reached (checked against the replay). */
export interface SecretRealmClaim {
	endTick: number;
	kills: number;
	score: number;
	stateHash: number;
	/** [direction 0..8, ticks] — 0 = still, 1..8 = N, NE, E, SE, S, SW, W, NW */
	moves: [number, number][];
}

export interface SecretRealmOutcome {
	won: boolean;
	/** survived | fallen | gave_up */
	reason: string;
	endTick: number;
	gameMillis: number;
	kills: number;
	score: number;
	qi: number;
	level: number;
	hpLeft: number;
	spawned: number;
	stateHash: number;
	runs: number;
	changes: number;
	maxWindowChanges: number;
	changesPerSecMilli: number;
	movingTicks: number;
	claimedEndTick: number;
	elapsedMillis: number;
	slowSubmit?: boolean;
	/** review only (never a rejection): mostly_idle | low_change | periodic */
	reviewFlags?: string[];
}

export interface SecretRealmSubmit {
	runId: string;
	realmId: string;
	/** pass = survived, fail = fell or stopped (both earn by band), rejected, expired */
	verdict: TrialVerdict;
	reason: string;
	outcome: SecretRealmOutcome | null;
	band: SecretRealmBand | null;
	loot: SecretRealmLoot | null;
	vipPoints: number;
	replayed: boolean;
	status: SecretRealmStatus;
}

// ───── pháp bảo slot (GET /artifacts, POST /artifacts/equip) ─────

export interface EquippedArtifact {
	/** "" = none */
	itemId: string;
	tribulationBonusBp: number;
	equippedAt: string | null;
}

// ───── hộ pháp (GET /protector, invite/accept, POST /presence/ping) ─────

export interface ProtectorView {
	serverNow: string;
	factionId: string;
	/** the viewer stands before a tribulation (may invite) */
	eligible: boolean;
	pingEverySeconds: number;
	inviteWindowSeconds: number;
	dailyAssists: number;
	assistsLeft: number;
	maxCounted: number;
	perMemberBp: number;
	maxBp: number;
	myInvite: {
		inviteId: string;
		stageId: string;
		createdAt: string;
		expiresAt: string;
		protectors: { characterId: string; name: string; acceptedAt: string; online: boolean; sameFaction: boolean; counts: boolean }[];
		counted: number;
	} | null;
	openInvites: { inviteId: string; characterId: string; name: string; stageId: string; expiresAt: string; protectors: number; accepted: boolean }[];
	onlineMembers: { characterId: string; name: string; stageId: string }[];
	online: boolean;
}

// ───── nạp Tiên Duyên (GET /payments/packages, /payments/orders…; Phase 4) ─────

export type PaymentStatus = 'created' | 'pending' | 'paid' | 'failed' | 'expired' | 'refunded';

/** Points are computed server-side from vip.yaml (1.000đ = pointsPer1000Vnd); no bonus. */
export interface PaymentPackage {
	id: string;
	name: string;
	amountVnd: number;
	points: number;
}

export interface PaymentPackagesView {
	currency: string;
	pointName: string;
	pointsPer1000Vnd: number;
	minAmountVnd: number;
	maxAmountVnd: number;
	orderExpiryMinutes: number;
	packages: PaymentPackage[];
	/** enabled on this server; empty → top-up closed */
	providers: { id: string; name: string }[];
}

export interface PaymentOrder {
	id: string;
	packageId: string;
	packageName: string;
	amountVnd: number;
	points: number;
	provider: string;
	status: PaymentStatus;
	/** only while the order is open */
	checkout?: { url: string; qr?: string };
	failureReason?: string;
	createdAt: string;
	expiresAt: string;
	paidAt?: string;
	closedAt?: string;
	refundedAt?: string;
}

export interface PaymentOrderResult {
	order: PaymentOrder;
	/** current Tiên Duyên (GET /payments/orders/{id}) */
	vip?: VipStatus;
	replayed?: boolean;
}

export interface PaymentWebhookResult {
	outcome: 'applied' | 'replayed' | 'ignored' | 'rejected';
	detail?: string;
	orderId: string;
	status?: PaymentStatus;
}

/** DEV ONLY: POST /dev/payments/sandbox/{orderId} */
export interface SandboxCompleteResult {
	eventId: string;
	delivered: boolean;
	webhook?: PaymentWebhookResult;
	order: PaymentOrder;
}

export interface AdminPaymentOrder extends PaymentOrder {
	userId: string;
	providerRef?: string;
	vipLedgerId?: number;
	refundLedgerId?: number;
	checkCount: number;
	lastCheckedAt?: string;
}

export interface PaymentEvent {
	id: number;
	provider: string;
	dedupeKey: string;
	source: 'webhook' | 'reconcile';
	signatureOk: boolean;
	orderId?: string;
	kind: 'paid' | 'failed' | 'expired' | 'refunded' | 'pending';
	amountVnd?: number;
	outcome: 'received' | 'applied' | 'replayed' | 'ignored' | 'rejected';
	detail?: string;
	receivedAt: string;
}

export interface AdminPaymentRefundResult {
	order: AdminPaymentOrder;
	refund: { userId: string; ledgerId: number; refundLedgerId: number; pointsRemoved: number; pointsBefore: number; pointsAfter: number; replayed: boolean };
	vip: VipStatus;
	replayed: boolean;
}

// ───── Đạo lữ song tu (Phase 5.1) — GET /dao-lu, /dao-lu/song-tu… ─────

export type DaoLuBlocker = 'account_age' | 'tier' | 'bonded' | 'cooldown';

export interface DaoLuRules {
	minAccountAgeDays: number;
	minTier: number;
	proposalExpiryHours: number;
	dailyProposals: number;
	absenceDays: number;
	rebondCooldownDays: number;
	windowSeconds: number;
	rateMilliPerHour: number;
	heartbeatSeconds: number;
	maxSongTuMilli: number;
	duyenMinSeconds: number;
}

/** incoming: characterId is the proposer · outgoing: the target */
export interface DaoLuProposal {
	proposalId: string;
	characterId: string;
	name: string;
	stageId: string;
	createdAt: string;
	expiresAt: string;
}

export interface DaoLuBond {
	bondId: string;
	formedAt: string;
	partner: { characterId: string; name: string; stageId: string; online: boolean };
}

export interface SongTuView {
	available: boolean;
	gameDay: string;
	resetAt: string;
	sessionStarted: boolean;
	/** shared overlap today (both partners active, server clock) */
	accruedSeconds: number;
	/** my paid seconds today */
	paidSeconds: number;
	claimableSeconds: number;
	/** raw tu vi of claimableSeconds — the bucket cap / 150 ceiling may clip it */
	claimableMilli: number;
	windowSeconds: number;
	meActive: boolean;
	partnerActive: boolean;
	partnerBeatAt: string | null;
	heartbeatSeconds: number;
}

export interface DaoLuView {
	serverNow: string;
	rules: DaoLuRules;
	blockers: DaoLuBlocker[];
	cooldownUntil: string | null;
	proposalsLeft: number;
	bond: DaoLuBond | null;
	incoming: DaoLuProposal[];
	outgoing: DaoLuProposal[];
	songTu: SongTuView | null;
}

export interface SongTuClaimResult {
	cultivation: CultivationSnapshot;
	seconds: number;
	rawMilli: number;
	earnedMilli: number;
	vipPoints: number;
	replayed: boolean;
	songTu: SongTuView;
}

// ───── Trao đổi vật phẩm (Phase 5.2) — GET/POST /trades, trade.yaml ─────
export type TradeRelation = 'stranger' | 'faction' | 'dao_lu';
export type TradeBlocker = 'account_age' | 'character_age' | 'tier';
export type TradeStatus = 'pending' | 'accepted' | 'declined' | 'cancelled' | 'expired';

/** An intent line: which asset (spirit_stone or a trade.yaml item) and how many. The server prices and moves it. */
export interface TradeLineInput {
	assetId: string;
	quantity: number;
}
export interface TradeLine extends TradeLineInput {
	kind: 'currency' | 'item';
	/** stone-equivalent per unit (trade.yaml) */
	unitValue: number;
}
export interface TradeOffer {
	id: string;
	kind: 'gift' | 'swap';
	status: TradeStatus;
	/** incoming = I am the receiver */
	direction: 'incoming' | 'outgoing';
	counterpart: { characterId: string; name: string; stageId: string };
	/** sender → receiver */
	give: TradeLine[];
	/** receiver → sender (swap) */
	take: TradeLine[];
	valueToReceiver: number;
	valueToSender: number;
	/** linh thạch burned from the sender (pending: if accepted now) */
	senderFee: number;
	/** linh thạch burned from the receiver (swap) */
	receiverFee: number;
	relation: TradeRelation;
	createdAt: string;
	expiresAt: string;
	respondedAt: string | null;
}
export interface TradeGate {
	minAccountAgeDays: number;
	minCharacterAgeDays: number;
	minTier: number;
}
export interface TradeRules {
	assets: { id: string; kind: 'currency' | 'item'; value: number }[];
	stoneFeeBp: number;
	minStoneFee: number;
	expiryHours: number;
	maxLinesPerSide: number;
	maxQuantityPerLine: number;
	maxOpenOutgoing: number;
	offersPerDay: number;
	completedPerDay: number;
	sentValuePerDay: number;
	receivedValuePerDay: { stranger: number; faction: number; daoLu: number };
	send: TradeGate;
	receive: TradeGate;
}
export interface TradeLimits {
	gameDay: string;
	resetAt: string;
	offersLeft: number;
	completedLeft: number;
	openOutgoing: number;
	valueReceived: number;
	valueSent: number;
	sendBlockers: TradeBlocker[];
	receiveBlockers: TradeBlocker[];
}
export interface TradeView {
	serverNow: string;
	rules: TradeRules;
	limits: TradeLimits;
	incoming: TradeOffer[];
	outgoing: TradeOffer[];
	replayed?: boolean;
}
export interface TradePeer {
	characterId: string;
	name: string;
	stageId: string;
	relation: TradeRelation;
	/** daily received-value cap between us (by relation) */
	receiveCap: number;
	/** why they cannot receive now */
	blockers: TradeBlocker[];
}

// ───── tutorial (Vân Khê; content/story/tutorial/*.yaml, decisions 2026-11-09 #1) ─────
// Display only: a step never grants anything. Steps fire from server facts (fxBus after a server answer, server
// previews) or places the player opened — never client guesses.

/** Expression = art slot characters/guide/<expr> (SVG fallback draws each). */
export type GuideExpr = 'neutral' | 'smile' | 'point' | 'worried' | 'proud' | 'surprised';
export interface TutorialLine {
	expr: GuideExpr;
	/** ≤ 200 characters, numbers already filled from the catalogues */
	text: string;
	/** data-tutorial="<id>" target to highlight */
	spotlight?: string;
}
export interface TutorialStep {
	id: string;
	/** onboarding = /create before a character exists (client-only); play = stored on the server */
	flow: 'onboarding' | 'play';
	trigger: string;
	title: string;
	lines: TutorialLine[];
}
export interface TutorialScript {
	guide: string;
	name: string;
	steps: TutorialStep[];
}
export interface TutorialState {
	done: string[];
	skipped: string[];
	/** "không nhắc lại" */
	muted: boolean;
}
export interface TutorialMarkResult {
	replayed: boolean;
	state: TutorialState;
}

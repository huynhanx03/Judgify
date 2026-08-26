"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ArrowDown, ArrowUp, Loader2, RefreshCw, RotateCcw, ShieldCheck } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { AUTHORIZATION_ACTION, AUTHORIZATION_RESOURCE } from "@/constants/authorization";
import { OPERATION_STATUS } from "@/constants/operation";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { useAuth } from "@/contexts/auth-context";
import { getErrorMessage, notify } from "@/lib/toast";
import { operationStatusLabel } from "@/lib/operations/presentation";
import { formatDateTime, formatNumber } from "@/lib/format";
import { progressionService } from "@/services/progression.service";
import type {
	ProgressionAdjustmentPreview,
	ProgressionLedgerEntry,
	RewardRule,
	RewardRulePublicationPreview,
	RankingReconciliationPreview,
	RankingReconciliationStatus,
} from "@/types/progression";

export default function AdminProgressionPage() {
	const { can } = useAuth();
	const canReadLedger = can(AUTHORIZATION_RESOURCE.CULTIVATION_LEDGER, AUTHORIZATION_ACTION.READ);
	const canAdjust = can(AUTHORIZATION_RESOURCE.CULTIVATION_LEDGER, AUTHORIZATION_ACTION.ADJUST);
	const canReverse = can(AUTHORIZATION_RESOURCE.CULTIVATION_LEDGER, AUTHORIZATION_ACTION.REVERSE);
	const canRebuild = canReadLedger && can(AUTHORIZATION_RESOURCE.CULTIVATION_LEDGER, AUTHORIZATION_ACTION.REBUILD);
	const canReadRules = can(AUTHORIZATION_RESOURCE.CULTIVATION_REWARD_RULE, AUTHORIZATION_ACTION.READ);
	const canPublishRules = can(AUTHORIZATION_RESOURCE.CULTIVATION_REWARD_RULE, AUTHORIZATION_ACTION.PUBLISH);
	const defaultTab = canReadLedger ? "ledger" : "rules";

	const [userID, setUserID] = useState("");
	const [entries, setEntries] = useState<ProgressionLedgerEntry[]>([]);
	const [ledgerCursor, setLedgerCursor] = useState<string>();
	const [ledgerLoading, setLedgerLoading] = useState(false);
	const [ledgerLoaded, setLedgerLoaded] = useState(false);
	const [delta, setDelta] = useState("");
	const [incidentID, setIncidentID] = useState("");
	const [reason, setReason] = useState("");
	const [reversalEntry, setReversalEntry] = useState<ProgressionLedgerEntry>();
	const [adjustmentPreview, setAdjustmentPreview] = useState<ProgressionAdjustmentPreview>();
	const [adjustmentBusy, setAdjustmentBusy] = useState(false);
	const [reconciliationStatus, setReconciliationStatus] = useState<RankingReconciliationStatus>();
	const [reconciliationPreview, setReconciliationPreview] = useState<RankingReconciliationPreview>();
	const [reconciliationReason, setReconciliationReason] = useState("");
	const [reconciliationBusy, setReconciliationBusy] = useState(false);

	const [rules, setRules] = useState<RewardRule[]>([]);
	const [rulesCursor, setRulesCursor] = useState<number>();
	const [rulesLoading, setRulesLoading] = useState(false);
	const [rulesError, setRulesError] = useState<string | null>(null);
	const [multiplier, setMultiplier] = useState("10000");
	const [flatBonus, setFlatBonus] = useState("0");
	const [ruleReason, setRuleReason] = useState("");
	const [rulePreview, setRulePreview] = useState<RewardRulePublicationPreview>();
	const [ruleBusy, setRuleBusy] = useState(false);

	const reversedIDs = useMemo(
		() => new Set(entries.flatMap((entry) => entry.reversed_entry_id ? [entry.reversed_entry_id] : [])),
		[entries],
	);

	const loadLedger = useCallback(async (cursor?: string) => {
		if (!canReadLedger || !userID.trim()) return;
		setLedgerLoading(true);
		try {
			const page = await progressionService.listLedger(userID.trim(), cursor);
			setEntries((current) => cursor ? [...current, ...page.entries] : page.entries);
			setLedgerCursor(page.next_cursor);
			setLedgerLoaded(true);
		} catch (error) {
			notify.error(getErrorMessage(error, ADMIN_TEXT.PROGRESSION.LEDGER_LOAD_ERROR));
		} finally {
			setLedgerLoading(false);
		}
	}, [canReadLedger, userID]);

	const loadRules = useCallback(async (cursor?: number) => {
		if (!canReadRules) return;
		setRulesLoading(true);
		setRulesError(null);
		try {
			const page = await progressionService.listRewardRules(cursor);
			setRules((current) => cursor ? [...current, ...page.rules] : page.rules);
			setRulesCursor(page.next_cursor);
		} catch (error) {
			const message = getErrorMessage(error, ADMIN_TEXT.PROGRESSION.RULE_LOAD_ERROR);
			setRulesError(message);
			if (cursor) notify.error(message);
		} finally {
			setRulesLoading(false);
		}
	}, [canReadRules]);

	const loadReconciliationStatus = useCallback(async () => {
		if (!canReadLedger) return;
		setReconciliationBusy(true);
		try {
			setReconciliationStatus(await progressionService.rankingReconciliationStatus());
		} catch (error) {
			notify.error(getErrorMessage(error, ADMIN_TEXT.PROGRESSION.RECONCILIATION_STATUS_ERROR));
		} finally {
			setReconciliationBusy(false);
		}
	}, [canReadLedger]);

	useEffect(() => {
		if (canReadRules) void loadRules();
	}, [canReadRules, loadRules]);

	useEffect(() => {
		if (canReadLedger) void loadReconciliationStatus();
	}, [canReadLedger, loadReconciliationStatus]);

	async function startRankingReconciliation() {
		if (!canRebuild || reconciliationBusy || !reconciliationPreview) return;
		setReconciliationBusy(true);
		try {
			const started = await progressionService.startRankingReconciliation(
				reconciliationReason,
				reconciliationPreview,
			);
			setReconciliationStatus((current) => ({
				projection_revision: current?.projection_revision ?? 0,
				last_reconciled_at: current?.last_reconciled_at,
				latest_operation: started.operation,
			}));
			setReconciliationReason("");
			setReconciliationPreview(undefined);
			notify.success(started.idempotent_replay
				? ADMIN_TEXT.PROGRESSION.RECONCILIATION_REPLAYED
				: ADMIN_TEXT.PROGRESSION.RECONCILIATION_STARTED);
		} catch (error) {
			notify.error(getErrorMessage(error, ADMIN_TEXT.PROGRESSION.RECONCILIATION_START_ERROR));
		} finally {
			setReconciliationBusy(false);
		}
	}

	async function previewRankingReconciliation() {
		if (!canRebuild || reconciliationBusy) return;
		setReconciliationBusy(true);
		try {
			setReconciliationPreview(await progressionService.rankingReconciliationPreview());
		} catch (error) {
			setReconciliationPreview(undefined);
			notify.error(getErrorMessage(error, ADMIN_TEXT.PROGRESSION.RECONCILIATION_PREVIEW_ERROR));
		} finally {
			setReconciliationBusy(false);
		}
	}

	async function previewAdjustment() {
		setAdjustmentBusy(true);
		try {
			const preview = await progressionService.previewAdjustment({
				user_id: userID.trim(),
				delta: reversalEntry ? 0 : Number(delta),
				reversed_entry_id: reversalEntry?.id,
			});
			setAdjustmentPreview(preview);
		} catch (error) {
			notify.error(getErrorMessage(error, ADMIN_TEXT.PROGRESSION.PREVIEW_ERROR));
		} finally {
			setAdjustmentBusy(false);
		}
	}

	async function applyAdjustment() {
		if (!adjustmentPreview) return;
		setAdjustmentBusy(true);
		try {
			await progressionService.applyAdjustment({
				user_id: adjustmentPreview.user_id,
				delta: adjustmentPreview.delta,
				expected_balance: adjustmentPreview.balance_before,
				expected_version: adjustmentPreview.expected_version,
				incident_id: incidentID.trim(), reason: reason.trim(),
				preview_checksum: adjustmentPreview.preview_checksum,
				reversed_entry_id: adjustmentPreview.reversed_entry_id,
			});
			notify.success(reversalEntry
				? ADMIN_TEXT.PROGRESSION.REVERSAL_SUCCESS
				: ADMIN_TEXT.PROGRESSION.ADJUSTMENT_SUCCESS);
			setAdjustmentPreview(undefined);
			setReversalEntry(undefined);
			setDelta(""); setIncidentID(""); setReason("");
			await loadLedger();
		} catch (error) {
			notify.error(getErrorMessage(error, ADMIN_TEXT.PROGRESSION.APPLY_ERROR));
		} finally {
			setAdjustmentBusy(false);
		}
	}

	function startReversal(entry: ProgressionLedgerEntry) {
		setReversalEntry(entry);
		setAdjustmentPreview(undefined);
		setDelta("");
		const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
		document.getElementById("progression-command")?.scrollIntoView({
			behavior: reduceMotion ? "auto" : "smooth",
			block: "center",
		});
	}

	async function previewRule() {
		setRuleBusy(true);
		try {
			setRulePreview(await progressionService.previewRewardRule({
				multiplier_basis_points: Number(multiplier), flat_bonus: Number(flatBonus),
			}));
		} catch (error) {
			notify.error(getErrorMessage(error, ADMIN_TEXT.PROGRESSION.RULE_PREVIEW_ERROR));
		} finally { setRuleBusy(false); }
	}

	async function publishRule() {
		if (!rulePreview) return;
		setRuleBusy(true);
		try {
			await progressionService.publishRewardRule({
				expected_revision: rulePreview.current_revision,
				algorithm_version: rulePreview.algorithm_version,
				configuration: rulePreview.configuration,
				reason: ruleReason.trim(), preview_checksum: rulePreview.preview_checksum,
			});
			notify.success(ADMIN_TEXT.PROGRESSION.RULE_PUBLISH_SUCCESS);
			setRulePreview(undefined); setRuleReason("");
			await loadRules();
		} catch (error) {
			notify.error(getErrorMessage(error, ADMIN_TEXT.PROGRESSION.RULE_PUBLISH_ERROR));
		} finally { setRuleBusy(false); }
	}

	return (
		<div className="space-y-6">
			<header className="flex flex-col gap-2">
				<div className="flex items-center gap-2 text-primary">
					<ShieldCheck className="size-5" aria-hidden="true" />
					<span className="text-xs font-semibold uppercase tracking-[0.18em]">{ADMIN_TEXT.PROGRESSION.EYEBROW}</span>
				</div>
				<h1 className="text-2xl font-bold tracking-tight sm:text-3xl">{ADMIN_TEXT.PROGRESSION.TITLE}</h1>
				<p className="max-w-3xl text-sm leading-6 text-muted-foreground">{ADMIN_TEXT.PROGRESSION.SUBTITLE}</p>
			</header>

			<Tabs defaultValue={defaultTab}>
				<TabsList aria-label={ADMIN_TEXT.PROGRESSION.TABS_LABEL}>
					{canReadLedger ? <TabsTrigger value="ledger">{ADMIN_TEXT.PROGRESSION.LEDGER_TAB}</TabsTrigger> : null}
					{canReadRules ? <TabsTrigger value="rules">{ADMIN_TEXT.PROGRESSION.RULE_TAB}</TabsTrigger> : null}
				</TabsList>

				{canReadLedger ? (
					<TabsContent value="ledger" className="space-y-5">
						<Card className="border-primary/20">
							<CardHeader className="flex flex-row items-start justify-between gap-4">
								<div>
									<CardTitle>{ADMIN_TEXT.PROGRESSION.RECONCILIATION_TITLE}</CardTitle>
									<CardDescription>{ADMIN_TEXT.PROGRESSION.RECONCILIATION_DESC}</CardDescription>
								</div>
								<Button type="button" variant="ghost" size="sm" disabled={reconciliationBusy} onClick={() => void loadReconciliationStatus()} aria-label={ADMIN_TEXT.PROGRESSION.RECONCILIATION_REFRESH}>
									<RefreshCw className={reconciliationBusy ? "size-4 animate-spin motion-reduce:animate-none" : "size-4"} aria-hidden="true" />
								</Button>
							</CardHeader>
							<CardContent className="space-y-4">
								<div className="grid gap-3 text-sm sm:grid-cols-3">
									<div className="rounded-lg border bg-muted/30 p-3"><p className="text-xs text-muted-foreground">{ADMIN_TEXT.PROGRESSION.RECONCILIATION_REVISION}</p><p className="mt-1 font-semibold tabular-nums">{reconciliationStatus?.projection_revision ?? 0}</p></div>
									<div className="rounded-lg border bg-muted/30 p-3"><p className="text-xs text-muted-foreground">{ADMIN_TEXT.PROGRESSION.LAST_RECONCILED}</p><p className="mt-1 font-semibold">{reconciliationStatus?.last_reconciled_at ? formatDateTime(reconciliationStatus.last_reconciled_at) : ADMIN_TEXT.PROGRESSION.NEVER_RECONCILED}</p></div>
									<div className="rounded-lg border bg-muted/30 p-3"><p className="text-xs text-muted-foreground">{ADMIN_TEXT.PROGRESSION.LATEST_RECONCILIATION}</p>{reconciliationStatus?.latest_operation ? <div className="mt-1 flex items-center gap-2"><Badge variant="outline">{operationStatusLabel(reconciliationStatus.latest_operation.status)}</Badge><span className="text-xs tabular-nums">{formatNumber(reconciliationStatus.latest_operation.processed)} / {formatNumber(reconciliationStatus.latest_operation.total)}</span></div> : <p className="mt-1 font-semibold">{ADMIN_TEXT.PROGRESSION.NO_RECONCILIATION}</p>}</div>
								</div>
								{canRebuild ? (
									<div className="grid gap-3 border-t pt-4">
										<div className="space-y-2"><Label htmlFor="ranking-reconciliation-reason">{ADMIN_TEXT.PROGRESSION.RECONCILIATION_REASON}</Label><Textarea id="ranking-reconciliation-reason" value={reconciliationReason} maxLength={1024} onChange={(event) => setReconciliationReason(event.target.value)} placeholder={ADMIN_TEXT.PROGRESSION.RECONCILIATION_REASON_PLACEHOLDER} /></div>
										{reconciliationPreview ? (
											<div className="grid gap-2 rounded-lg border border-primary/20 bg-primary/5 p-4 text-sm sm:grid-cols-3" role="status">
												<span>{ADMIN_TEXT.PROGRESSION.RECONCILIATION_SOURCE_COUNT}: <strong>{formatNumber(reconciliationPreview.source_count)}</strong></span>
												<span>{ADMIN_TEXT.PROGRESSION.RECONCILIATION_REPAIR_COUNT}: <strong>{formatNumber(reconciliationPreview.repair_count)}</strong></span>
												<span>{ADMIN_TEXT.PROGRESSION.RECONCILIATION_PREVIEW_REVISION}: <strong>{formatNumber(reconciliationPreview.projection_revision)}</strong></span>
												<span className="sm:col-span-3">{ADMIN_TEXT.PROGRESSION.RECONCILIATION_PREVIEW_CUTOFF}: <strong>{formatDateTime(reconciliationPreview.source_cutoff)}</strong></span>
											</div>
										) : null}
										<div className="flex flex-wrap justify-end gap-2">
											<Button type="button" variant="outline" disabled={reconciliationBusy || reconciliationStatus?.latest_operation?.status === OPERATION_STATUS.PENDING || reconciliationStatus?.latest_operation?.status === OPERATION_STATUS.RUNNING || reconciliationStatus?.latest_operation?.status === OPERATION_STATUS.PAUSED} onClick={() => void previewRankingReconciliation()}>{ADMIN_TEXT.PROGRESSION.PREVIEW}</Button>
											<Button type="button" disabled={reconciliationBusy || !reconciliationPreview || reconciliationReason.trim().length < 3 || reconciliationStatus?.latest_operation?.status === OPERATION_STATUS.PENDING || reconciliationStatus?.latest_operation?.status === OPERATION_STATUS.RUNNING || reconciliationStatus?.latest_operation?.status === OPERATION_STATUS.PAUSED} onClick={() => void startRankingReconciliation()}>{reconciliationBusy ? <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" /> : null}{ADMIN_TEXT.PROGRESSION.CONFIRM_RECONCILIATION}</Button>
										</div>
									</div>
								) : null}
							</CardContent>
						</Card>

						<Card>
							<CardHeader>
								<CardTitle>{ADMIN_TEXT.PROGRESSION.LEDGER_QUERY_TITLE}</CardTitle>
								<CardDescription>{ADMIN_TEXT.PROGRESSION.LEDGER_QUERY_DESC}</CardDescription>
							</CardHeader>
							<CardContent>
								<form className="flex flex-col gap-3 sm:flex-row" onSubmit={(event) => { event.preventDefault(); void loadLedger(); }}>
									<div className="flex-1 space-y-2">
										<Label htmlFor="progression-user-id">{ADMIN_TEXT.PROGRESSION.USER_ID}</Label>
										<Input id="progression-user-id" value={userID} onChange={(event) => { setUserID(event.target.value); setLedgerLoaded(false); setAdjustmentPreview(undefined); }} placeholder={ADMIN_TEXT.PROGRESSION.USER_ID_PLACEHOLDER} autoComplete="off" />
									</div>
									<Button className="sm:mt-7" disabled={ledgerLoading || !userID.trim()}>
										{ledgerLoading ? <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" /> : null}
										{ADMIN_TEXT.PROGRESSION.SEARCH}
									</Button>
								</form>
							</CardContent>
						</Card>

						{(canAdjust || reversalEntry) && ledgerLoaded ? (
							<Card id="progression-command" className="border-primary/20">
								<CardHeader>
									<CardTitle>{reversalEntry ? ADMIN_TEXT.PROGRESSION.REVERSAL_TITLE : ADMIN_TEXT.PROGRESSION.ADJUSTMENT_TITLE}</CardTitle>
									<CardDescription>{reversalEntry ? ADMIN_TEXT.PROGRESSION.REVERSAL_DESC : ADMIN_TEXT.PROGRESSION.ADJUSTMENT_DESC}</CardDescription>
								</CardHeader>
								<CardContent className="grid gap-4 md:grid-cols-2">
									{reversalEntry ? (
										<div className="rounded-lg border bg-muted/40 p-3 text-sm md:col-span-2">
											<span className="font-medium">{ADMIN_TEXT.PROGRESSION.REVERSING_ENTRY}</span> <span className="font-mono text-xs">{reversalEntry.id}</span>
											<Button variant="ghost" size="sm" className="ml-2" onClick={() => { setReversalEntry(undefined); setAdjustmentPreview(undefined); }}>{TEXT.COMMON.CANCEL}</Button>
										</div>
									) : (
										<div className="space-y-2">
											<Label htmlFor="progression-delta">{ADMIN_TEXT.PROGRESSION.DELTA}</Label>
											<Input id="progression-delta" type="number" min={-1_000_000} max={1_000_000} value={delta} onChange={(event) => { setDelta(event.target.value); setAdjustmentPreview(undefined); }} />
										</div>
									)}
									<div className="space-y-2">
										<Label htmlFor="progression-incident">{ADMIN_TEXT.PROGRESSION.INCIDENT_ID}</Label>
										<Input id="progression-incident" value={incidentID} onChange={(event) => setIncidentID(event.target.value)} placeholder={ADMIN_TEXT.PROGRESSION.INCIDENT_PLACEHOLDER} />
									</div>
									<div className="space-y-2 md:col-span-2">
										<Label htmlFor="progression-reason">{ADMIN_TEXT.PROGRESSION.REASON}</Label>
										<Textarea id="progression-reason" value={reason} onChange={(event) => setReason(event.target.value)} placeholder={ADMIN_TEXT.PROGRESSION.REASON_PLACEHOLDER} />
									</div>
									{adjustmentPreview ? (
										<div className="grid gap-2 rounded-lg border border-primary/20 bg-primary/5 p-4 text-sm md:col-span-2 sm:grid-cols-3" role="status">
											<span>{ADMIN_TEXT.PROGRESSION.BEFORE}: <strong>{formatNumber(adjustmentPreview.balance_before)}</strong></span>
											<span>{ADMIN_TEXT.PROGRESSION.DELTA}: <strong>{adjustmentPreview.delta > 0 ? "+" : ""}{formatNumber(adjustmentPreview.delta)}</strong></span>
											<span>{ADMIN_TEXT.PROGRESSION.AFTER}: <strong>{formatNumber(adjustmentPreview.balance_after)}</strong></span>
										</div>
									) : null}
									<div className="flex flex-wrap justify-end gap-2 md:col-span-2">
										<Button variant="outline" disabled={adjustmentBusy || (!reversalEntry && !delta)} onClick={() => void previewAdjustment()}>{ADMIN_TEXT.PROGRESSION.PREVIEW}</Button>
										<Button disabled={adjustmentBusy || !adjustmentPreview || incidentID.trim().length < 3 || reason.trim().length < 3} onClick={() => void applyAdjustment()}>{adjustmentBusy ? <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" /> : null}{reversalEntry ? ADMIN_TEXT.PROGRESSION.CONFIRM_REVERSAL : ADMIN_TEXT.PROGRESSION.CONFIRM_ADJUSTMENT}</Button>
									</div>
								</CardContent>
							</Card>
						) : null}

						<Card>
							<CardHeader><CardTitle>{ADMIN_TEXT.PROGRESSION.HISTORY_TITLE}</CardTitle><CardDescription>{ADMIN_TEXT.PROGRESSION.HISTORY_DESC}</CardDescription></CardHeader>
							<CardContent className="space-y-3">
								{ledgerLoaded && entries.length === 0 ? <p className="py-10 text-center text-sm text-muted-foreground">{ADMIN_TEXT.PROGRESSION.LEDGER_EMPTY}</p> : null}
								{entries.map((entry) => {
									const reversible = canReverse && entry.kind === "manual_adjustment" && !reversedIDs.has(entry.id);
									return <article key={entry.id} className="grid gap-3 rounded-xl border p-4 lg:grid-cols-[minmax(0,1fr)_auto]">
										<div className="min-w-0 space-y-2">
											<div className="flex flex-wrap items-center gap-2"><Badge variant="outline">{entry.kind}</Badge><span className="text-xs text-muted-foreground">v{entry.progression_version} · {formatDateTime(entry.occurred_at)}</span></div>
											<div className="flex items-center gap-2 text-base font-semibold">{entry.delta >= 0 ? <ArrowUp className="size-4 text-primary" aria-hidden="true" /> : <ArrowDown className="size-4 text-destructive" aria-hidden="true" />}<span className="tabular-nums">{entry.delta >= 0 ? "+" : ""}{formatNumber(entry.delta)}</span><span className="text-sm font-normal text-muted-foreground">{formatNumber(entry.balance_before)} → {formatNumber(entry.balance_after)}</span></div>
											<p className="truncate font-mono text-xs text-muted-foreground" title={entry.correlation_id}>{ADMIN_TEXT.PROGRESSION.CORRELATION_ID} · {entry.correlation_id}</p>
											{entry.incident_id || entry.reason ? <p className="text-sm text-muted-foreground">{entry.incident_id ? `${entry.incident_id} · ` : ""}{entry.reason}</p> : null}
										</div>
										{reversible ? <Button variant="outline" size="sm" onClick={() => startReversal(entry)}><RotateCcw className="size-4" aria-hidden="true" />{ADMIN_TEXT.PROGRESSION.REVERSE}</Button> : null}
									</article>;
								})}
								{ledgerCursor ? <Button variant="outline" className="w-full" disabled={ledgerLoading} onClick={() => void loadLedger(ledgerCursor)}>{ledgerLoading ? <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" /> : null}{ADMIN_TEXT.PROGRESSION.LOAD_MORE}</Button> : null}
							</CardContent>
						</Card>
					</TabsContent>
				) : null}

				{canReadRules ? (
					<TabsContent value="rules" className="space-y-5">
						{canPublishRules ? <Card className="border-primary/20"><CardHeader><CardTitle>{ADMIN_TEXT.PROGRESSION.RULE_PUBLISH_TITLE}</CardTitle><CardDescription>{ADMIN_TEXT.PROGRESSION.RULE_PUBLISH_DESC}</CardDescription></CardHeader><CardContent className="grid gap-4 md:grid-cols-2">
							<div className="space-y-2"><Label htmlFor="rule-multiplier">{ADMIN_TEXT.PROGRESSION.MULTIPLIER}</Label><Input id="rule-multiplier" type="number" min={0} max={100000} value={multiplier} onChange={(event) => { setMultiplier(event.target.value); setRulePreview(undefined); }} /></div>
							<div className="space-y-2"><Label htmlFor="rule-flat-bonus">{ADMIN_TEXT.PROGRESSION.FLAT_BONUS}</Label><Input id="rule-flat-bonus" type="number" min={-1000000000} max={1000000000} value={flatBonus} onChange={(event) => { setFlatBonus(event.target.value); setRulePreview(undefined); }} /></div>
							<div className="space-y-2 md:col-span-2"><Label htmlFor="rule-reason">{ADMIN_TEXT.PROGRESSION.REASON}</Label><Textarea id="rule-reason" value={ruleReason} onChange={(event) => setRuleReason(event.target.value)} placeholder={ADMIN_TEXT.PROGRESSION.RULE_REASON_PLACEHOLDER} /></div>
							{rulePreview ? <div className="rounded-lg border border-primary/20 bg-primary/5 p-4 text-sm md:col-span-2" role="status">{ADMIN_TEXT.PROGRESSION.RULE_PREVIEW_SUMMARY(rulePreview.current_revision, rulePreview.next_revision)}</div> : null}
							<div className="flex justify-end gap-2 md:col-span-2"><Button variant="outline" disabled={ruleBusy} onClick={() => void previewRule()}>{ADMIN_TEXT.PROGRESSION.PREVIEW}</Button><Button disabled={ruleBusy || !rulePreview || ruleReason.trim().length < 3} onClick={() => void publishRule()}>{ruleBusy ? <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" /> : null}{ADMIN_TEXT.PROGRESSION.PUBLISH_RULE}</Button></div>
						</CardContent></Card> : null}
		<Card><CardHeader><CardTitle>{ADMIN_TEXT.PROGRESSION.RULE_HISTORY_TITLE}</CardTitle><CardDescription>{ADMIN_TEXT.PROGRESSION.RULE_HISTORY_DESC}</CardDescription></CardHeader><CardContent className="space-y-3">
							{rulesLoading && rules.length === 0 ? <div className="flex items-center gap-2 py-10 text-sm text-muted-foreground"><Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />{ADMIN_TEXT.PROGRESSION.LOADING}</div> : null}
							{rulesError ? <div className="flex flex-col items-start gap-3 rounded-xl border border-destructive/25 bg-destructive/5 p-4 text-sm text-destructive" role="alert"><span>{rulesError}</span><Button type="button" size="sm" variant="outline" disabled={rulesLoading} onClick={() => void loadRules()}>{ADMIN_TEXT.PROGRESSION.RETRY}</Button></div> : null}
							{!rulesLoading && !rulesError && rules.length === 0 ? <p className="py-10 text-center text-sm text-muted-foreground">{ADMIN_TEXT.PROGRESSION.RULE_EMPTY}</p> : null}
							{rules.map((rule) => <article key={rule.id} className="grid gap-3 rounded-xl border p-4 sm:grid-cols-[1fr_auto]"><div><div className="flex items-center gap-2"><Badge>{ADMIN_TEXT.PROGRESSION.REVISION(rule.revision)}</Badge><span className="text-sm font-medium">{rule.algorithm_version}</span></div><p className="mt-2 text-sm text-muted-foreground">{ADMIN_TEXT.PROGRESSION.RULE_FORMULA(rule.configuration.multiplier_basis_points, rule.configuration.flat_bonus)}</p><p className="mt-2 truncate font-mono text-xs text-muted-foreground" title={rule.checksum}>{rule.checksum}</p></div><time className="text-xs text-muted-foreground">{formatDateTime(rule.effective_from)}</time></article>)}
							{rulesCursor ? <Button variant="outline" className="w-full" disabled={rulesLoading} onClick={() => void loadRules(rulesCursor)}>{rulesLoading ? <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" /> : <RefreshCw className="size-4" aria-hidden="true" />}{ADMIN_TEXT.PROGRESSION.LOAD_MORE}</Button> : null}
						</CardContent></Card>
					</TabsContent>
				) : null}
			</Tabs>
		</div>
	);
}

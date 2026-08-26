"use client";

import { useMemo, useState } from "react";
import {
	AlertTriangle,
	CheckCircle2,
	Link2,
	Loader2,
	RefreshCw,
	ShieldCheck,
	Unlink,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { OAUTH_PROVIDERS } from "@/constants/identity";
import { TEXT } from "@/constants/text";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { ApiError } from "@/lib/api/error";
import { formatDateTime } from "@/lib/format";
import { OAuthProviderIcon } from "@/modules/auth/oauth-provider-icon";
import { authService } from "@/services/auth.service";
import type {
	OAuthIdentity,
	OAuthIdentityListResponse,
	OAuthProvider,
} from "@/types/auth";

type AccountAction =
	| { kind: "connect" | "disconnect"; provider: OAuthProvider }
	| null;

const EMPTY_IDENTITIES: OAuthIdentityListResponse = { items: [] };

function safeDate(value: string | undefined): string {
	if (!value) return TEXT.COMMON.NOT_AVAILABLE;
	const parsed = new Date(value);
	return Number.isNaN(parsed.getTime())
		? TEXT.COMMON.UNKNOWN
		: formatDateTime(value);
}

function providerName(provider: OAuthProvider): string {
	return TEXT.AUTH.OAUTH_PROVIDER_NAME[provider];
}

function actionErrorMessage(error: unknown): string {
	if (error instanceof ApiError && error.status === 409) {
		return TEXT.PROFILE.OAUTH_ACCOUNTS.LAST_METHOD_ERROR;
	}
	if (error instanceof ApiError && error.status === 404) {
		return TEXT.PROFILE.OAUTH_ACCOUNTS.NOT_FOUND;
	}
	return TEXT.PROFILE.OAUTH_ACCOUNTS.DISCONNECT_ERROR;
}

export function OAuthAccountsCard() {
	const [action, setAction] = useState<AccountAction>(null);
	const [selectedProvider, setSelectedProvider] = useState<OAuthProvider | null>(null);
	const [message, setMessage] = useState<
		{ tone: "error" | "success"; text: string } | null
	>(null);

	const identitiesResource =
		useRetryableResource<OAuthIdentityListResponse>({
			resetKey: "profile-oauth-identities",
			initialData: EMPTY_IDENTITIES,
			load: (signal) => authService.listOAuthIdentities(signal),
		});
	const projection = identitiesResource.data;

	const identityByProvider = useMemo(
		() => new Map(projection.items.map((identity) => [identity.provider, identity])),
		[projection.items],
	);

	async function connect(provider: OAuthProvider) {
		if (action) return;
		setMessage(null);
		setAction({ kind: "connect", provider });
		try {
			const response = await authService.startOAuthLink(provider);
			window.location.assign(response.auth_url);
		} catch {
			setMessage({ tone: "error", text: TEXT.PROFILE.OAUTH_ACCOUNTS.CONNECT_ERROR });
			setAction(null);
		}
	}

	async function disconnect() {
		if (!selectedProvider || action) return;
		const provider = selectedProvider;
		setMessage(null);
		setAction({ kind: "disconnect", provider });
		try {
			await authService.unlinkOAuth(provider);
			setSelectedProvider(null);
			identitiesResource.retry();
			setMessage({
				tone: "success",
				text: TEXT.PROFILE.OAUTH_ACCOUNTS.DISCONNECT_SUCCESS(providerName(provider)),
			});
		} catch (error) {
			setMessage({ tone: "error", text: actionErrorMessage(error) });
			if (error instanceof ApiError && error.status === 404) {
				setSelectedProvider(null);
				identitiesResource.retry();
			}
		} finally {
			setAction(null);
		}
	}

	return (
		<>
			<Card className="glass-card overflow-hidden border-border/40">
				<CardHeader className="gap-3 border-b border-border/60 bg-muted/15 sm:flex-row sm:items-start sm:justify-between">
					<div className="flex min-w-0 gap-3">
						<div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
							<ShieldCheck className="size-5" aria-hidden="true" />
						</div>
						<div>
							<CardTitle className="text-lg">
								{TEXT.PROFILE.OAUTH_ACCOUNTS.TITLE}
							</CardTitle>
							<CardDescription className="mt-1 leading-5">
								{TEXT.PROFILE.OAUTH_ACCOUNTS.DESCRIPTION}
							</CardDescription>
						</div>
					</div>
				</CardHeader>
				<CardContent className="space-y-4 p-4 sm:p-5">
					{message ? (
						<div
							className={
								message.tone === "error"
									? "flex items-start gap-2 rounded-xl border border-destructive/25 bg-destructive/10 p-3 text-sm text-destructive"
									: "flex items-start gap-2 rounded-xl border border-success/25 bg-success/10 p-3 text-sm text-success"
							}
							role={message.tone === "error" ? "alert" : "status"}
						>
							{message.tone === "error" ? (
								<AlertTriangle className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
							) : (
								<CheckCircle2 className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
							)}
							<span>{message.text}</span>
						</div>
					) : null}

					{identitiesResource.status === "loading" ? (
						<div className="space-y-3" role="status" aria-busy="true">
							<span className="sr-only">{TEXT.COMMON.LOADING}</span>
							{OAUTH_PROVIDERS.map((provider) => (
								<div
									key={provider}
									className="h-24 animate-pulse rounded-2xl bg-muted/50 motion-reduce:animate-none"
								/>
							))}
						</div>
					) : identitiesResource.status === "error" ? (
						<div className="flex min-h-36 flex-col items-center justify-center text-center" role="alert">
							<AlertTriangle className="size-5 text-destructive" aria-hidden="true" />
							<p className="mt-3 font-medium">
								{TEXT.PROFILE.OAUTH_ACCOUNTS.LOAD_ERROR_TITLE}
							</p>
							<p className="mt-1 max-w-sm text-sm text-muted-foreground">
								{TEXT.PROFILE.OAUTH_ACCOUNTS.LOAD_ERROR_DESCRIPTION}
							</p>
							<Button
								type="button"
								variant="outline"
								className="mt-4 min-h-11"
								onClick={identitiesResource.retry}
							>
								<RefreshCw aria-hidden="true" />
								{TEXT.PROFILE.OAUTH_ACCOUNTS.RETRY}
							</Button>
						</div>
					) : (
						<ul className="grid gap-3" aria-label={TEXT.PROFILE.OAUTH_ACCOUNTS.TITLE}>
							{OAUTH_PROVIDERS.map((provider) => {
								const identity: OAuthIdentity | undefined = identityByProvider.get(provider);
								const busy = action?.provider === provider;
								const name = providerName(provider);
								return (
									<li
										key={provider}
										className="rounded-2xl border border-border/70 bg-background/70 p-4 transition-colors duration-200 hover:border-primary/35 hover:bg-muted/25 motion-reduce:transition-none"
									>
										<div className="flex flex-col gap-4 sm:flex-row sm:items-center">
											<div className="flex min-w-0 flex-1 items-start gap-3">
												<div className="flex size-11 shrink-0 items-center justify-center rounded-xl border border-border/70 bg-card shadow-sm">
													<OAuthProviderIcon provider={provider} className="size-5" />
												</div>
												<div className="min-w-0 flex-1">
													<div className="flex flex-wrap items-center gap-2">
														<p className="font-semibold">{name}</p>
														<Badge variant={identity ? "default" : "secondary"}>
															{identity
																? TEXT.PROFILE.OAUTH_ACCOUNTS.CONNECTED
																: TEXT.PROFILE.OAUTH_ACCOUNTS.NOT_CONNECTED}
														</Badge>
													</div>
													{identity ? (
														<dl className="mt-2 grid gap-1 text-xs text-muted-foreground sm:grid-cols-2">
															{identity.display_name ? (
																<div className="sm:col-span-2">
																	<dt className="inline">{TEXT.PROFILE.OAUTH_ACCOUNTS.ACCOUNT_LABEL}: </dt>
																	<dd className="inline font-medium text-foreground/80">{identity.display_name}</dd>
																</div>
															) : null}
															<div>
																<dt className="inline">{TEXT.PROFILE.OAUTH_ACCOUNTS.LINKED_AT}: </dt>
																<dd className="inline tabular-nums text-foreground/80">{safeDate(identity.linked_at)}</dd>
															</div>
															<div>
																<dt className="inline">{TEXT.PROFILE.OAUTH_ACCOUNTS.LAST_USED}: </dt>
																<dd className="inline tabular-nums text-foreground/80">{safeDate(identity.last_authenticated_at)}</dd>
															</div>
														</dl>
													) : null}
												</div>
											</div>
											<Button
												type="button"
												variant={identity ? "outline" : "default"}
												className="min-h-11 w-full shrink-0 sm:w-auto"
												disabled={Boolean(action)}
												onClick={() => {
													if (identity) {
														setMessage(null);
														setSelectedProvider(provider);
													} else {
														void connect(provider);
													}
												}}
											>
												{busy ? (
													<Loader2 className="animate-spin motion-reduce:animate-none" aria-hidden="true" />
												) : identity ? (
													<Unlink aria-hidden="true" />
												) : (
													<Link2 aria-hidden="true" />
												)}
												{busy && action?.kind === "connect"
													? TEXT.PROFILE.OAUTH_ACCOUNTS.CONNECTING
													: identity
														? TEXT.PROFILE.OAUTH_ACCOUNTS.DISCONNECT
														: TEXT.PROFILE.OAUTH_ACCOUNTS.CONNECT}
											</Button>
										</div>
									</li>
								);
							})}
						</ul>
					)}
					<p className="border-t border-border/60 pt-4 text-xs leading-5 text-muted-foreground">
						{TEXT.PROFILE.OAUTH_ACCOUNTS.SECURITY_NOTE}
					</p>
				</CardContent>
			</Card>

			<ConfirmDialog
				open={selectedProvider !== null}
				onOpenChange={(open) => {
					if (!open && !action) setSelectedProvider(null);
				}}
				onConfirm={() => void disconnect()}
				title={TEXT.PROFILE.OAUTH_ACCOUNTS.DISCONNECT_TITLE}
				description={
					selectedProvider
						? TEXT.PROFILE.OAUTH_ACCOUNTS.DISCONNECT_DESCRIPTION(
								providerName(selectedProvider),
							)
						: TEXT.PROFILE.OAUTH_ACCOUNTS.DISCONNECT_ERROR
				}
				confirmLabel={TEXT.PROFILE.OAUTH_ACCOUNTS.DISCONNECT_CONFIRM}
				loading={action?.kind === "disconnect"}
			/>
		</>
	);
}

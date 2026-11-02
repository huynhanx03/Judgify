"use client";

import { type FormEvent, useMemo, useState } from "react";
import {
  CircleAlert,
  Eye,
  FilePenLine,
  LoaderCircle,
  RefreshCw,
  Send,
  ShieldCheck,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import {
  CAMPAIGN_AUDIENCE,
  CAMPAIGN_LIMITS,
} from "@/constants/notification-campaign";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { tryEntityID } from "@/lib/api/contracts";
import { MarkdownRenderer } from "@/modules/shared/markdown-renderer";
import type { Role } from "@/types/admin";
import type {
  CampaignAudienceKind,
  CreateCampaignInput,
  NotificationCampaign,
} from "@/types/notification-campaign";

interface CampaignComposerDialogProps {
  open: boolean;
  campaign?: NotificationCampaign | null;
  roles: Role[];
  rolesLoading: boolean;
  rolesError: boolean;
  onRetryRoles(): void;
  canBroadcastRole: boolean;
  canBroadcastAllActive: boolean;
  pending: boolean;
  error?: string | null;
  onOpenChange(open: boolean): void;
  onSave(input: CreateCampaignInput): Promise<void>;
}

const encoder = new TextEncoder();

function byteLength(value: string): number {
  return encoder.encode(value).byteLength;
}

function initialAudience(
  campaign?: NotificationCampaign | null,
): CampaignAudienceKind {
  return campaign?.revision.audience.kind ?? CAMPAIGN_AUDIENCE.EXPLICIT_USERS;
}

export function CampaignComposerDialog({
  open,
  campaign,
  roles,
  rolesLoading,
  rolesError,
  onRetryRoles,
  canBroadcastRole,
  canBroadcastAllActive,
  pending,
  error,
  onOpenChange,
  onSave,
}: CampaignComposerDialogProps) {
  const [title, setTitle] = useState(
    () => campaign?.revision.title ?? "",
  );
  const [body, setBody] = useState(
    () => campaign?.revision.body_markdown ?? "",
  );
  const [actionPath, setActionPath] = useState(
    () => campaign?.revision.action_path ?? "",
  );
  const [reason, setReason] = useState("");
  const [audienceKind, setAudienceKind] =
    useState<CampaignAudienceKind>(() => initialAudience(campaign));
  const [roleID, setRoleID] = useState(
    () => campaign?.revision.audience.role_id ?? "",
  );
  const [explicitUsers, setExplicitUsers] = useState(
    () => campaign?.revision.audience.explicit_users?.join("\n") ?? "",
  );
  const [validationError, setValidationError] = useState<string | null>(null);

  const userIDs = useMemo(
    () =>
      Array.from(
        new Set(
          explicitUsers
            .split(/[\s,;]+/)
            .map((value) => value.trim())
            .filter(Boolean),
        ),
      ),
    [explicitUsers],
  );
  const parsedRoleID = roleID ? tryEntityID(roleID) : null;
  const parsedUserIDs = userIDs.map(tryEntityID);
  const explicitUsersValid = parsedUserIDs.every((id) => id !== null);

  const titleBytes = byteLength(title.trim());
  const bodyBytes = byteLength(body.trim());
  const reasonBytes = byteLength(reason.trim());

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setValidationError(null);
    if (
      titleBytes < 1 ||
      titleBytes > CAMPAIGN_LIMITS.TITLE_BYTES ||
      bodyBytes < 1 ||
      bodyBytes > CAMPAIGN_LIMITS.BODY_BYTES ||
      reasonBytes < 1 ||
      reasonBytes > CAMPAIGN_LIMITS.REASON_BYTES
    ) {
      setValidationError(ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.INVALID);
      return;
    }
    if (
      (audienceKind === CAMPAIGN_AUDIENCE.EXPLICIT_USERS &&
        (userIDs.length < 1 ||
          userIDs.length > CAMPAIGN_LIMITS.EXPLICIT_USERS ||
          !explicitUsersValid)) ||
      (audienceKind === CAMPAIGN_AUDIENCE.ROLE &&
        (!canBroadcastRole || rolesError || parsedRoleID === null)) ||
      (audienceKind === CAMPAIGN_AUDIENCE.ALL_ACTIVE &&
        !canBroadcastAllActive)
    ) {
      setValidationError(
        ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.AUDIENCE_INVALID,
      );
      return;
    }
    try {
      await onSave({
        title,
        body_markdown: body,
        ...(actionPath.trim() ? { action_path: actionPath } : {}),
        audience: {
          kind: audienceKind,
          ...(audienceKind === CAMPAIGN_AUDIENCE.ROLE
            ? { role_id: parsedRoleID! }
            : {}),
          ...(audienceKind === CAMPAIGN_AUDIENCE.EXPLICIT_USERS
            ? { user_ids: parsedUserIDs.filter((id) => id !== null) }
            : {}),
        },
        reason,
      });
    } catch {
      // The parent exposes the normalized API error without clearing fields.
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-5xl">
        <DialogHeader>
          <div className="mb-1 flex size-11 items-center justify-center rounded-xl border border-primary/20 bg-primary/10 text-primary">
            <FilePenLine className="size-5" aria-hidden="true" />
          </div>
          <DialogTitle>
            {campaign
              ? ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.EDIT_TITLE
              : ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.CREATE_TITLE}
          </DialogTitle>
          <DialogDescription>
            {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.DESCRIPTION}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={submit} className="space-y-5">
          <Tabs defaultValue="compose">
            <TabsList className="w-full sm:w-auto">
              <TabsTrigger value="compose">
                <FilePenLine className="size-4" aria-hidden="true" />
                {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.COMPOSE_TAB}
              </TabsTrigger>
              <TabsTrigger value="preview">
                <Eye className="size-4" aria-hidden="true" />
                {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.PREVIEW_TAB}
              </TabsTrigger>
            </TabsList>
            <TabsContent value="compose" className="mt-4 space-y-5">
              <div className="grid gap-5 lg:grid-cols-[minmax(0,1.5fr)_minmax(18rem,0.7fr)]">
                <div className="space-y-5">
                  <div className="space-y-2">
                    <div className="flex items-center justify-between gap-3">
                      <Label htmlFor="campaign-title">
                        {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.TITLE}
                      </Label>
                      <span className="text-xs tabular-nums text-muted-foreground">
                        {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.BYTE_COUNT(
                          titleBytes,
                          CAMPAIGN_LIMITS.TITLE_BYTES,
                        )}
                      </span>
                    </div>
                    <Input
                      id="campaign-title"
                      value={title}
                      onChange={(event) => setTitle(event.target.value)}
                      placeholder={
                        ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.TITLE_PLACEHOLDER
                      }
                      autoFocus
                    />
                  </div>

                  <div className="space-y-2">
                    <div className="flex items-center justify-between gap-3">
                      <Label htmlFor="campaign-body">
                        {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.BODY}
                      </Label>
                      <span className="text-xs tabular-nums text-muted-foreground">
                        {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.BYTE_COUNT(
                          bodyBytes,
                          CAMPAIGN_LIMITS.BODY_BYTES,
                        )}
                      </span>
                    </div>
                    <Textarea
                      id="campaign-body"
                      value={body}
                      onChange={(event) => setBody(event.target.value)}
                      placeholder={
                        ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.BODY_PLACEHOLDER
                      }
                      className="min-h-72 resize-y font-mono text-sm leading-6"
                    />
                    <p className="text-xs leading-5 text-muted-foreground">
                      {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.BODY_HINT}
                    </p>
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="campaign-action">
                      {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.ACTION_PATH}
                    </Label>
                    <Input
                      id="campaign-action"
                      value={actionPath}
                      onChange={(event) => setActionPath(event.target.value)}
                      placeholder={
                        ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.ACTION_PLACEHOLDER
                      }
                      className="font-mono"
                    />
                    <p className="text-xs leading-5 text-muted-foreground">
                      {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.ACTION_HINT}
                    </p>
                  </div>
                </div>

                <aside className="space-y-5 rounded-2xl border border-border bg-muted/25 p-4">
                  <div>
                    <div className="flex items-center gap-2 text-sm font-semibold">
                      <ShieldCheck className="size-4 text-primary" aria-hidden="true" />
                      {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.AUDIENCE_TITLE}
                    </div>
                    <p className="mt-1 text-xs leading-5 text-muted-foreground">
                      {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.AUDIENCE_DESCRIPTION}
                    </p>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="campaign-audience">
                      {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.AUDIENCE_KIND}
                    </Label>
                    <Select
                      value={audienceKind}
                      onValueChange={(value) =>
                        setAudienceKind(value as CampaignAudienceKind)
                      }
                    >
                      <SelectTrigger id="campaign-audience" className="w-full">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value={CAMPAIGN_AUDIENCE.EXPLICIT_USERS}>
                          {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.AUDIENCE.EXPLICIT_USERS}
                        </SelectItem>
                        <SelectItem
                          value={CAMPAIGN_AUDIENCE.ROLE}
                          disabled={!canBroadcastRole}
                        >
                          {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.AUDIENCE.ROLE}
                        </SelectItem>
                        <SelectItem
                          value={CAMPAIGN_AUDIENCE.ALL_ACTIVE}
                          disabled={!canBroadcastAllActive}
                        >
                          {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.AUDIENCE.ALL_ACTIVE}
                        </SelectItem>
                      </SelectContent>
                    </Select>
                  </div>

                  {audienceKind === CAMPAIGN_AUDIENCE.ROLE ? (
                    <div className="space-y-2">
                      <Label htmlFor="campaign-role">
                        {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.ROLE}
                      </Label>
                      <Select
                        value={roleID}
                        onValueChange={setRoleID}
                        disabled={rolesLoading || rolesError}
                      >
                        <SelectTrigger id="campaign-role" className="w-full">
                          <SelectValue
                            placeholder={
                              rolesLoading
                                ? ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.ROLES_LOADING
                                : ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.ROLE_PLACEHOLDER
                            }
                          />
                        </SelectTrigger>
                        <SelectContent>
                          {roles.map((role) => (
                            <SelectItem key={role.id} value={role.id}>
                              {role.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      {rolesError ? (
                        <div
                          className="flex items-start gap-2 rounded-lg border border-destructive/20 bg-destructive/5 p-3"
                          role="alert"
                        >
                          <CircleAlert
                            className="mt-0.5 size-4 shrink-0 text-destructive"
                            aria-hidden="true"
                          />
                          <p className="min-w-0 flex-1 text-xs leading-5 text-muted-foreground">
                            {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.ROLES_LOAD_ERROR}
                          </p>
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon-sm"
                            onClick={onRetryRoles}
                            aria-label={
                              ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.RETRY_ROLES
                            }
                          >
                            <RefreshCw className="size-4" aria-hidden="true" />
                          </Button>
                        </div>
                      ) : null}
                    </div>
                  ) : null}

                  {audienceKind === CAMPAIGN_AUDIENCE.EXPLICIT_USERS ? (
                    <div className="space-y-2">
                      <div className="flex items-center justify-between gap-3">
                        <Label htmlFor="campaign-users">
                          {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.USER_IDS}
                        </Label>
                        <span className="text-xs tabular-nums text-muted-foreground">
                          {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.USER_COUNT(
                            userIDs.length,
                            CAMPAIGN_LIMITS.EXPLICIT_USERS,
                          )}
                        </span>
                      </div>
                      <Textarea
                        id="campaign-users"
                        value={explicitUsers}
                        onChange={(event) =>
                          setExplicitUsers(event.target.value)
                        }
                        placeholder={
                          ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.USER_IDS_PLACEHOLDER
                        }
                        className="min-h-36 resize-y font-mono text-xs"
                      />
                      <p className="text-xs leading-5 text-muted-foreground">
                        {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.USER_IDS_HINT}
                      </p>
                    </div>
                  ) : null}

                  {audienceKind !== CAMPAIGN_AUDIENCE.EXPLICIT_USERS ? (
                    <div className="rounded-xl border border-warning/30 bg-warning/10 p-3 text-xs leading-5 text-foreground">
                      {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.BROADCAST_WARNING}
                    </div>
                  ) : null}
                </aside>
              </div>

              <div className="space-y-2">
                <div className="flex items-center justify-between gap-3">
                  <Label htmlFor="campaign-reason">
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.REASON}
                  </Label>
                  <span className="text-xs tabular-nums text-muted-foreground">
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.BYTE_COUNT(
                      reasonBytes,
                      CAMPAIGN_LIMITS.REASON_BYTES,
                    )}
                  </span>
                </div>
                <Textarea
                  id="campaign-reason"
                  value={reason}
                  onChange={(event) => setReason(event.target.value)}
                  placeholder={
                    ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.REASON_PLACEHOLDER
                  }
                  className="min-h-24 resize-y"
                />
              </div>
            </TabsContent>

            <TabsContent value="preview" className="mt-4">
              <div className="min-h-96 rounded-2xl border border-border bg-card p-5 sm:p-7">
                {title.trim() || body.trim() ? (
                  <>
                    <p className="text-xs font-semibold tracking-[0.14em] text-primary uppercase">
                      {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.LOCAL_PREVIEW}
                    </p>
                    <h2 className="mt-3 text-2xl font-semibold tracking-tight text-foreground">
                      {title || ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.UNTITLED}
                    </h2>
                    <MarkdownRenderer content={body} className="mt-5" />
                  </>
                ) : (
                  <div className="flex min-h-80 items-center justify-center text-center text-sm text-muted-foreground">
                    {ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.PREVIEW_EMPTY}
                  </div>
                )}
              </div>
            </TabsContent>
          </Tabs>

          {validationError || error ? (
            <p role="alert" className="text-sm text-destructive">
              {validationError ?? error}
            </p>
          ) : null}

          <DialogFooter className="sticky bottom-0 z-10 bg-background/95 supports-backdrop-filter:backdrop-blur">
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={pending}
            >
              {TEXT.COMMON.CANCEL}
            </Button>
            <Button type="submit" disabled={pending}>
              {pending ? (
                <LoaderCircle
                  className="size-4 animate-spin motion-reduce:animate-none"
                  aria-hidden="true"
                />
              ) : (
                <Send className="size-4" aria-hidden="true" />
              )}
              {pending
                ? ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.SAVING
                : ADMIN_TEXT.NOTIFICATION_CAMPAIGNS.COMPOSER.SAVE}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

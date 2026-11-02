"use client";

import { useEffect, useMemo, useState, type FormEvent } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { useTenant } from "@/contexts/tenant-context";
import { useLocale } from "@/contexts/locale-context";
import { tenantService } from "@/services/tenant.service";
import type { TenantInvitation, TenantMember } from "@/types/tenant";

type DialogMode = "create" | "rename" | "invite" | null;

function remainingLabel(nextAt: string, now: number): string {
  const remaining = Math.max(0, new Date(nextAt).getTime() - now);
  const days = Math.floor(remaining / 86_400_000);
  const hours = Math.floor((remaining % 86_400_000) / 3_600_000);
  return days > 0 ? `${days} ngày ${hours} giờ` : `${hours} giờ`;
}

export function TenantSelector({ mobile = false }: { mobile?: boolean }) {
  const { t } = useLocale();
  const {
    tenants,
    selectedTenantId,
    isAllTenants,
    loading,
    selectTenant,
    createTenant,
    renameTenant,
  } = useTenant();
  const [dialogMode, setDialogMode] = useState<DialogMode>(null);
  const [name, setName] = useState("");
  const [username, setUsername] = useState("");
  const [invitations, setInvitations] = useState<TenantInvitation[]>([]);
  const [members, setMembers] = useState<TenantMember[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [now, setNow] = useState(Date.now());
  const current = tenants.find((tenant) => tenant.id === selectedTenantId);
  const nextNameChangeAt = current?.next_name_change_at;
  const remaining = useMemo(
    () => nextNameChangeAt ? remainingLabel(nextNameChangeAt, now) : "",
    [nextNameChangeAt, now],
  );

  useEffect(() => {
    if (!nextNameChangeAt) return;
    const timer = window.setInterval(() => setNow(Date.now()), 60_000);
    return () => window.clearInterval(timer);
  }, [nextNameChangeAt]);

  useEffect(() => {
    const controller = new AbortController();
    void tenantService.findPendingInvitationsForUser(controller.signal).then((result) => {
      setInvitations(result.invitations);
    }).catch(() => {});
    return () => controller.abort();
  }, []);

  useEffect(() => {
    if (!current || current.role !== "owner" || isAllTenants) {
      setMembers([]);
      return;
    }
    const controller = new AbortController();
    void tenantService.findMembersForOwner(current.id, controller.signal).then((result) => {
      setMembers(result.members);
    }).catch(() => {});
    return () => controller.abort();
  }, [current?.id, current?.role, isAllTenants]);

  const closeDialog = () => {
    setDialogMode(null);
    setName("");
    setUsername("");
    setError("");
  };

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const normalized = name.trim();
    if (dialogMode === "invite" ? !username.trim() : !normalized) return;
    setSaving(true);
    setError("");
    try {
      if (dialogMode === "create") await createTenant(normalized);
      else if (dialogMode === "rename" && current) {
        await renameTenant(current.id, normalized);
      } else if (dialogMode === "invite" && current) {
        await tenantService.invite(current.id, username.trim());
      }
      closeDialog();
    } catch {
      setError(t("TENANT.ACTION_ERROR"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className={mobile ? "grid gap-2 border-b border-border/60 p-3" : "grid gap-2"}>
      <label className="sr-only" htmlFor={mobile ? "tenant-select-mobile" : "tenant-select"}>
        {t("TENANT.SELECT")}
      </label>
      <select
        id={mobile ? "tenant-select-mobile" : "tenant-select"}
        value={isAllTenants ? "" : selectedTenantId ?? ""}
        disabled={loading || tenants.length === 0}
        onChange={(event) => selectTenant(event.target.value || null)}
        className="min-h-10 min-w-40 rounded-lg border border-border bg-background px-3 text-sm text-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        {tenants.length > 0 && <option value="">{t("TENANT.ALL")}</option>}
        {tenants.map((tenant) => (
          <option key={tenant.id} value={tenant.id}>
            {tenant.name} · {tenant.role === "owner" ? t("TENANT.OWNER") : t("TENANT.MEMBER")}
          </option>
        ))}
      </select>
      {isAllTenants && (
        <p className="text-xs text-muted-foreground" role="status">
          {t("TENANT.ALL_READ_ONLY")}
        </p>
      )}
      <div className={mobile ? "flex flex-wrap items-center gap-2" : "flex items-center gap-2"}>
        <Button type="button" variant="outline" size="sm" onClick={() => setDialogMode("create")}>
          {t("TENANT.CREATE")}
        </Button>
        {current?.role === "owner" && !isAllTenants && (
          <>
          <Button type="button" variant="outline" size="sm" onClick={() => setDialogMode("invite")}>
            {t("TENANT.INVITE")}
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={!current.can_change_name}
            title={!current.can_change_name && remaining ? t("TENANT.RENAME_AVAILABLE", { remaining }) : undefined}
            onClick={() => setDialogMode("rename")}
          >
            {current.can_change_name ? t("TENANT.RENAME") : remaining ? t("TENANT.RENAME_AVAILABLE", { remaining }) : t("TENANT.RENAME")}
          </Button>
          </>
        )}
        {current?.role === "member" && tenants.length > 1 && !isAllTenants && (
          <Button type="button" variant="ghost" size="sm" onClick={async () => {
            if (!window.confirm(t("TENANT.LEAVE_CONFIRM"))) return;
            try { await tenantService.leave(current.id); window.location.reload(); }
            catch { setError(t("TENANT.ACTION_ERROR")); }
          }}>{t("TENANT.LEAVE")}</Button>
        )}
      </div>
      {invitations.length > 0 && (
        <div className="grid gap-2 rounded-lg border border-border/60 p-3">
          <p className="text-sm font-medium">{t("TENANT.INVITATIONS")}</p>
          {invitations.map((invitation) => (
            <div key={invitation.id} className="flex flex-wrap items-center justify-between gap-2 text-sm">
              <span>{invitation.tenant_name}</span>
              <Button type="button" size="sm" onClick={async () => {
                try {
                  const joined = await tenantService.acceptInvitation(invitation.id);
                  setInvitations((items) => items.filter((item) => item.id !== invitation.id));
                  await selectTenant(joined.id);
                } catch { setError(t("TENANT.ACTION_ERROR")); }
              }}>{t("TENANT.ACCEPT_INVITATION")}</Button>
            </div>
          ))}
        </div>
      )}
      {current?.role === "owner" && !isAllTenants && members.length > 0 && (
        <div className="grid gap-2 rounded-lg border border-border/60 p-3">
          <p className="text-sm font-medium">{t("TENANT.MANAGE_MEMBERS")}</p>
          {members.map((member) => (
            <div key={member.user_id} className="flex items-center justify-between gap-2 text-sm">
              <span>{member.username} · {member.role === "owner" ? t("TENANT.OWNER") : t("TENANT.MEMBER")}</span>
              {member.role === "member" && (
                <div className="flex items-center gap-1">
                <Button type="button" variant="ghost" size="sm" onClick={async () => {
                  if (!window.confirm(t("TENANT.TRANSFER_CONFIRM", { username: member.username }))) return;
                  try {
                    await tenantService.transferOwnership(current.id, member.user_id);
                    window.location.reload();
                  } catch { setError(t("TENANT.ACTION_ERROR")); }
                }}>{t("TENANT.TRANSFER_OWNER")}</Button>
                <Button type="button" variant="ghost" size="sm" onClick={async () => {
                  try {
                    await tenantService.removeMember(current.id, member.user_id);
                    setMembers((items) => items.filter((item) => item.user_id !== member.user_id));
                  } catch { setError(t("TENANT.ACTION_ERROR")); }
                }}>{t("TENANT.REMOVE_MEMBER")}</Button>
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      <Dialog open={dialogMode !== null} onOpenChange={(open) => !open && closeDialog()}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{dialogMode === "create" ? t("TENANT.CREATE_TITLE") : dialogMode === "invite" ? t("TENANT.INVITE_TITLE") : t("TENANT.RENAME_TITLE")}</DialogTitle>
            <DialogDescription>
              {dialogMode === "create" ? t("TENANT.CREATE_DESCRIPTION") : dialogMode === "invite" ? t("TENANT.INVITE_DESCRIPTION") : t("TENANT.RENAME_DESCRIPTION")}
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={submit} className="grid gap-4">
            {dialogMode === "invite" ? (
              <label className="grid gap-2 text-sm font-medium">
                {t("TENANT.USERNAME")}
                <input autoFocus required maxLength={50} value={username} onChange={(event) => setUsername(event.target.value)} className="min-h-11 rounded-lg border border-border bg-background px-3 font-normal outline-none focus-visible:ring-2 focus-visible:ring-ring" />
              </label>
            ) : <label className="grid gap-2 text-sm font-medium">
              {t("TENANT.NAME")}
              <input
                autoFocus
                required
                maxLength={120}
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder={t("TENANT.NAME_PLACEHOLDER")}
                className="min-h-11 rounded-lg border border-border bg-background px-3 font-normal outline-none focus-visible:ring-2 focus-visible:ring-ring"
              />
            </label>}
            {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
            <DialogFooter>
              <Button type="button" variant="outline" onClick={closeDialog}>{t("TENANT.CANCEL")}</Button>
              <Button type="submit" disabled={saving || (dialogMode === "invite" ? !username.trim() : !name.trim())}>
                {saving ? t("TENANT.SAVING") : t("TENANT.SAVE")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  );
}

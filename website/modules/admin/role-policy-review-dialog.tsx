"use client";

import { type FormEvent, useEffect, useState } from "react";
import {
  ArrowRight,
  CheckCircle2,
  KeyRound,
  Loader2,
  Minus,
  Plus,
  ShieldAlert,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
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
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { roleDisplayName } from "@/lib/auth/role-presentation";
import { formatDateTime } from "@/lib/format";
import type {
  PolicyRule,
  ReviewedRolePolicyCommand,
  Role,
} from "@/types/admin";

interface RolePolicyReviewDialogProps {
  open: boolean;
  role: Role | null;
  reviewed: ReviewedRolePolicyCommand | null;
  applying: boolean;
  reauthenticating: boolean;
  requiresReauthentication: boolean;
  operationError: string | null;
  onOpenChange(open: boolean): void;
  onConfirm(currentPassword: string): void;
}

function PolicyRuleList({
  rules,
  emptyMessage,
  tone,
}: {
  rules: PolicyRule[];
  emptyMessage: string;
  tone: "grant" | "revoke";
}) {
  if (rules.length === 0) {
    return <p className="text-sm text-muted-foreground">{emptyMessage}</p>;
  }

  return (
    <ul className="space-y-2">
      {rules.map((rule) => {
        const resource = ADMIN_TEXT.PERMISSIONS.RESOURCE_LABEL(rule.resource);
        const action = ADMIN_TEXT.PERMISSIONS.ACTION_LABEL(rule.action);
        return (
          <li
            key={`${rule.resource}\u0000${rule.action}`}
            className="flex min-h-11 items-start gap-3 rounded-xl border border-border bg-background/70 px-3 py-2.5"
          >
            <span
              className={
                tone === "grant"
                  ? "mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-lg bg-success/10 text-success"
                  : "mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-lg bg-destructive/10 text-destructive"
              }
              aria-hidden="true"
            >
              {tone === "grant" ? (
                <Plus className="size-3.5" strokeWidth={2.5} />
              ) : (
                <Minus className="size-3.5" strokeWidth={2.5} />
              )}
            </span>
            <span className="min-w-0">
              <span className="block text-sm font-semibold text-foreground">
                {action}
              </span>
              <span className="mt-0.5 block text-xs text-muted-foreground">
                {resource}
              </span>
              <code className="mt-1 block break-all text-[11px] text-primary">
                {rule.resource}/{rule.action}
              </code>
            </span>
          </li>
        );
      })}
    </ul>
  );
}

export function RolePolicyReviewDialog({
  open,
  role,
  reviewed,
  applying,
  reauthenticating,
  requiresReauthentication,
  operationError,
  onOpenChange,
  onConfirm,
}: RolePolicyReviewDialogProps) {
  const [password, setPassword] = useState("");
  const [passwordError, setPasswordError] = useState(false);
  const busy = applying || reauthenticating;

  useEffect(() => {
    if (!open) {
      setPassword("");
      setPasswordError(false);
    }
  }, [open]);

  useEffect(() => {
    if (!requiresReauthentication) {
      setPassword("");
      setPasswordError(false);
    }
  }, [requiresReauthentication]);

  if (!role || !reviewed) return null;

  const { preview } = reviewed.review;

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    if (requiresReauthentication && password.length === 0) {
      setPasswordError(true);
      return;
    }
    setPasswordError(false);
    onConfirm(password);
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!busy) onOpenChange(next);
      }}
    >
      <DialogContent
        className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-3xl"
        aria-busy={busy}
        showCloseButton={!busy}
      >
        <DialogHeader>
          <div className="mb-1 flex size-11 items-center justify-center rounded-xl border border-warning/30 bg-warning/10 text-warning">
            <ShieldAlert className="size-5" aria-hidden="true" />
          </div>
          <DialogTitle>{ADMIN_TEXT.ROLES_POLICY_REVIEW.TITLE}</DialogTitle>
          <DialogDescription>
            {ADMIN_TEXT.ROLES_POLICY_REVIEW.DESCRIPTION(
              roleDisplayName(role),
            )}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={submit} className="space-y-5">
          <section className="rounded-2xl border border-success/30 bg-success/10 p-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
                <CheckCircle2
                  className="size-4 text-success"
                  aria-hidden="true"
                />
                {ADMIN_TEXT.ROLES_POLICY_REVIEW.SERVER_REVIEWED}
              </div>
              <Badge variant="outline" className="bg-background/60 font-mono">
                {role.key}
              </Badge>
            </div>
            <p className="mt-2 text-xs leading-5 text-muted-foreground">
              {ADMIN_TEXT.ROLES_POLICY_REVIEW.EXPIRES_AT(
                formatDateTime(reviewed.review.expires_at),
              )}
            </p>
          </section>

          <dl className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <div className="rounded-xl border border-border bg-muted/25 p-3">
              <dt className="text-xs text-muted-foreground">
                {ADMIN_TEXT.ROLES_POLICY_REVIEW.CURRENT_REVISION}
              </dt>
              <dd className="mt-1 text-lg font-bold tabular-nums">
                {preview.expected_revision}
              </dd>
            </div>
            <div className="rounded-xl border border-border bg-muted/25 p-3">
              <dt className="text-xs text-muted-foreground">
                {ADMIN_TEXT.ROLES_POLICY_REVIEW.RESULTING_REVISION}
              </dt>
              <dd className="mt-1 flex items-center gap-2 text-lg font-bold tabular-nums">
                {preview.expected_revision}
                <ArrowRight className="size-4 text-muted-foreground" aria-hidden="true" />
                {preview.resulting_revision}
              </dd>
            </div>
            <div className="rounded-xl border border-success/25 bg-success/10 p-3">
              <dt className="text-xs text-muted-foreground">
                {ADMIN_TEXT.ROLES_POLICY_REVIEW.ADDED_COUNT}
              </dt>
              <dd className="mt-1 text-lg font-bold tabular-nums text-success">
                +{preview.added.length}
              </dd>
            </div>
            <div className="rounded-xl border border-destructive/25 bg-destructive/10 p-3">
              <dt className="text-xs text-muted-foreground">
                {ADMIN_TEXT.ROLES_POLICY_REVIEW.REMOVED_COUNT}
              </dt>
              <dd className="mt-1 text-lg font-bold tabular-nums text-destructive">
                -{preview.removed.length}
              </dd>
            </div>
          </dl>

          <div className="grid gap-4 lg:grid-cols-2">
            <section className="space-y-3 rounded-2xl border border-success/25 bg-success/5 p-4">
              <h3 className="flex items-center gap-2 text-sm font-semibold">
                <Plus className="size-4 text-success" aria-hidden="true" />
                {ADMIN_TEXT.ROLES_POLICY_REVIEW.ADDED_TITLE}
              </h3>
              <PolicyRuleList
                rules={preview.added}
                emptyMessage={ADMIN_TEXT.ROLES_POLICY_REVIEW.NO_ADDITIONS}
                tone="grant"
              />
            </section>
            <section className="space-y-3 rounded-2xl border border-destructive/25 bg-destructive/5 p-4">
              <h3 className="flex items-center gap-2 text-sm font-semibold">
                <Minus className="size-4 text-destructive" aria-hidden="true" />
                {ADMIN_TEXT.ROLES_POLICY_REVIEW.REMOVED_TITLE}
              </h3>
              <PolicyRuleList
                rules={preview.removed}
                emptyMessage={ADMIN_TEXT.ROLES_POLICY_REVIEW.NO_REMOVALS}
                tone="revoke"
              />
            </section>
          </div>

          <section className="rounded-2xl border border-border bg-muted/25 p-4">
            <h3 className="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
              {ADMIN_TEXT.ROLES_POLICY_REVIEW.REASON}
            </h3>
            <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-foreground">
              {reviewed.input.reason}
            </p>
          </section>

          <p className="rounded-xl border border-primary/20 bg-primary/5 px-4 py-3 text-xs leading-5 text-muted-foreground">
            {ADMIN_TEXT.ROLES_POLICY_REVIEW.IMMUTABLE_NOTICE}
          </p>

          {requiresReauthentication ? (
            <section className="space-y-3 rounded-2xl border border-warning/30 bg-warning/10 p-4">
              <div className="flex items-start gap-3">
                <KeyRound
                  className="mt-0.5 size-5 shrink-0 text-warning"
                  aria-hidden="true"
                />
                <div>
                  <h3 className="text-sm font-semibold text-foreground">
                    {ADMIN_TEXT.ROLES_POLICY_REVIEW.REAUTH_TITLE}
                  </h3>
                  <p className="mt-1 text-xs leading-5 text-muted-foreground">
                    {ADMIN_TEXT.ROLES_POLICY_REVIEW.REAUTH_DESCRIPTION}
                  </p>
                </div>
              </div>
              <div className="space-y-2">
                <Label htmlFor="role-policy-current-password">
                  {ADMIN_TEXT.ROLES_POLICY_REVIEW.CURRENT_PASSWORD}
                </Label>
                <Input
                  id="role-policy-current-password"
                  type="password"
                  autoComplete="current-password"
                  value={password}
                  maxLength={256}
                  disabled={busy}
                  aria-invalid={passwordError}
                  aria-describedby={
                    passwordError ? "role-policy-password-error" : undefined
                  }
                  placeholder={
                    ADMIN_TEXT.ROLES_POLICY_REVIEW.CURRENT_PASSWORD_PLACEHOLDER
                  }
                  onChange={(event) => {
                    setPassword(event.target.value);
                    setPasswordError(false);
                  }}
                />
                {passwordError ? (
                  <p
                    id="role-policy-password-error"
                    className="text-xs text-destructive"
                    role="alert"
                  >
                    {ADMIN_TEXT.ROLES_POLICY_REVIEW.PASSWORD_REQUIRED}
                  </p>
                ) : null}
              </div>
            </section>
          ) : null}

          {operationError ? (
            <div
              className="rounded-xl border border-destructive/25 bg-destructive/10 px-4 py-3 text-sm text-destructive"
              role="alert"
              aria-live="assertive"
            >
              {operationError}
            </div>
          ) : null}

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              className="min-h-11 cursor-pointer"
              disabled={busy}
              onClick={() => onOpenChange(false)}
            >
              {TEXT.COMMON.CANCEL}
            </Button>
            <Button
              type="submit"
              className="min-h-11 cursor-pointer"
              disabled={busy}
            >
              {busy ? (
                <Loader2
                  className="size-4 animate-spin motion-reduce:animate-none"
                  aria-hidden="true"
                />
              ) : (
                <CheckCircle2 className="size-4" aria-hidden="true" />
              )}
              {reauthenticating
                ? ADMIN_TEXT.ROLES_POLICY_REVIEW.REAUTHENTICATING
                : applying
                  ? ADMIN_TEXT.ROLES_POLICY_REVIEW.APPLYING
                  : ADMIN_TEXT.ROLES_POLICY_REVIEW.APPLY}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

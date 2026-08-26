"use client";

import { useEffect, useId, useRef } from "react";
import dynamic from "next/dynamic";
import { CircleAlert, RefreshCw, ShieldCheck } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { TEXT } from "@/constants/text";
import { cn } from "@/lib/utils";
import type { OnboardingProfileFormController } from "@/modules/auth/hooks/use-onboarding-profile-form";

/**
 * ProfileAttributeField is deferred: the fieldset below renders only once the
 * profile schema has resolved, and until then this component early-returns the
 * skeleton, so a field is never part of the first paint. Keeping it out of the
 * static graph keeps @base-ui's select (portal + positioner + list navigation)
 * out of /register's initial JS. `next/dynamic` has no `.preload()` in the App
 * Router, so the loader is held here and called on mount, racing the chunk
 * against the schema request that gates the fields.
 */
const importProfileAttributeField = () =>
  import("@/modules/profile/profile-attribute-field");

const ProfileAttributeField = dynamic(
  () => importProfileAttributeField().then((m) => m.ProfileAttributeField),
  { ssr: false },
);

interface OnboardingProfileFieldsProps {
  form: OnboardingProfileFormController;
  disabled?: boolean;
  className?: string;
}

export function OnboardingProfileFields({
  form,
  disabled = false,
  className,
}: OnboardingProfileFieldsProps) {
  const headingID = useId();
  const sectionRef = useRef<HTMLElement | null>(null);
  const refreshing = form.status === "loading" && Boolean(form.schema);
  const refreshFailed = form.status === "error" && Boolean(form.schema);

  useEffect(() => {
    void importProfileAttributeField();
  }, []);

  useEffect(() => {
    if (form.validationAttempt < 1) return;
    const invalidField = sectionRef.current?.querySelector<HTMLElement>(
      '[aria-invalid="true"]',
    );
    invalidField?.focus();
  }, [form.validationAttempt]);

  if ((form.status === "loading" || form.status === "idle") && !form.schema) {
    return (
      <section
        className={cn(
          "rounded-2xl border border-border/70 bg-card/60 p-4 sm:p-5",
          className,
        )}
        aria-busy="true"
        aria-label={TEXT.ONBOARDING_PROFILE.LOADING}
      >
        <div className="h-4 w-40 animate-pulse rounded bg-muted motion-reduce:animate-none" />
        <div className="mt-3 h-3 w-4/5 animate-pulse rounded bg-muted/80 motion-reduce:animate-none" />
        <div className="mt-5 grid gap-3 sm:grid-cols-2">
          {[0, 1, 2, 3].map((item) => (
            <div
              key={item}
              className="h-28 animate-pulse rounded-2xl border border-border/50 bg-muted/50 motion-reduce:animate-none"
            />
          ))}
        </div>
        <span className="sr-only">{TEXT.ONBOARDING_PROFILE.LOADING}</span>
      </section>
    );
  }

  if (!form.schema) {
    return (
      <section
        className={cn(
          "rounded-2xl border border-destructive/25 bg-destructive/5 p-4 sm:p-5",
          className,
        )}
        role="alert"
      >
        <div className="flex items-start gap-3">
          <CircleAlert className="mt-0.5 size-5 shrink-0 text-destructive" aria-hidden="true" />
          <div className="min-w-0 flex-1">
            <h3 className="font-semibold">{TEXT.ONBOARDING_PROFILE.LOAD_ERROR_TITLE}</h3>
            <p className="mt-1 text-sm leading-6 text-muted-foreground">
              {TEXT.ONBOARDING_PROFILE.LOAD_ERROR_DESCRIPTION}
            </p>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={form.retry}
              className="mt-4 min-h-11 gap-2 bg-background"
            >
              <RefreshCw className="size-4" aria-hidden="true" />
              {TEXT.COMMON.RETRY}
            </Button>
          </div>
        </div>
      </section>
    );
  }

  return (
    <section
      ref={sectionRef}
      className={cn(
        "rounded-2xl border border-border/70 bg-card/60 p-4 sm:p-5",
        className,
      )}
      aria-labelledby={headingID}
      aria-busy={refreshing}
    >
      <div className="flex flex-wrap items-start justify-between gap-3 border-b border-border/60 pb-4">
        <div className="min-w-0">
          <h3 id={headingID} className="flex items-center gap-2 font-semibold">
            <ShieldCheck className="size-4 text-primary" aria-hidden="true" />
            {TEXT.ONBOARDING_PROFILE.TITLE}
          </h3>
          <p className="mt-1 max-w-2xl text-xs leading-5 text-muted-foreground">
            {TEXT.ONBOARDING_PROFILE.DESCRIPTION}
          </p>
        </div>
        <Badge variant="outline" className="shrink-0 font-mono text-[10px]">
          {TEXT.ONBOARDING_PROFILE.REVISION(form.schema.revision)}
        </Badge>
      </div>

      {form.notice ? (
        <div
          className={cn(
            "mt-4 rounded-xl border px-3 py-2.5 text-xs leading-5",
            form.notice === "schema_reloaded"
              ? "border-warning/30 bg-warning/10 text-foreground"
              : "border-destructive/25 bg-destructive/5 text-destructive",
          )}
          role="status"
          aria-live="polite"
        >
          {form.notice === "schema_reloaded"
            ? TEXT.ONBOARDING_PROFILE.SCHEMA_RELOADED
            : TEXT.ONBOARDING_PROFILE.INVALID_VALUE}
        </div>
      ) : null}

      {refreshing ? (
        <div
          className="mt-4 flex items-center gap-2 rounded-xl border border-primary/20 bg-primary/5 px-3 py-2.5 text-xs text-muted-foreground"
          role="status"
          aria-live="polite"
        >
          <RefreshCw
            className="size-4 shrink-0 animate-spin text-primary motion-reduce:animate-none"
            aria-hidden="true"
          />
          {TEXT.ONBOARDING_PROFILE.REFRESHING}
        </div>
      ) : null}

      {refreshFailed ? (
        <div
          className="mt-4 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-destructive/25 bg-destructive/5 px-3 py-2.5 text-xs"
          role="alert"
        >
          <span className="text-muted-foreground">
            {TEXT.ONBOARDING_PROFILE.REFRESH_ERROR}
          </span>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={form.retry}
            className="min-h-11 gap-2 bg-background"
          >
            <RefreshCw className="size-4" aria-hidden="true" />
            {TEXT.COMMON.RETRY}
          </Button>
        </div>
      ) : null}

      <fieldset
        disabled={disabled || form.status !== "ready"}
        className="mt-4 grid min-w-0 gap-3 md:grid-cols-2"
      >
        <legend className="sr-only">{TEXT.ONBOARDING_PROFILE.TITLE}</legend>
        {form.attributes.map((attribute) => (
          <div
            key={attribute.definition_id}
            className="min-w-0"
            onBlur={(event) => {
              if (
                !event.currentTarget.contains(
                  event.relatedTarget as Node | null,
                )
              ) {
                form.validateField(attribute.definition_id);
              }
            }}
          >
            <ProfileAttributeField
              attribute={attribute}
              value={form.drafts[attribute.definition_id]}
              error={form.errors[attribute.definition_id]}
              disabled={disabled || form.status !== "ready"}
              onChange={(value) => form.updateDraft(attribute.definition_id, value)}
            />
          </div>
        ))}
      </fieldset>

      <p className="mt-4 text-xs leading-5 text-muted-foreground">
        {TEXT.ONBOARDING_PROFILE.SCHEMA_NOTE}
      </p>
    </section>
  );
}

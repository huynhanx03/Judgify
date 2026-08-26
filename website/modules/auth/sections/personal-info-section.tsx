"use client";

/**
 * Registration step 1 — personal info card.
 * Receives form state + handlers from RegisterFlow.
 */

import { Eye, EyeOff } from "lucide-react";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { IDENTITY_INPUT_LIMITS } from "@/constants/identity";
import { TEXT } from "@/constants/text";
import {
  isPasswordValid,
  isUsernameValid,
  usernameHTMLPattern,
} from "@/lib/auth/credentials";
import { isRecoveryEmailValid } from "@/lib/auth/recovery-email";

interface PersonalInfoSectionProps {
  form: {
    username: string;
    password: string;
    recovery_email: string;
  };
  updateField: (field: string, value: string) => void;
  showPassword: boolean;
  onTogglePassword: () => void;
  disabled?: boolean;
}

export function PersonalInfoSection({
  form,
  updateField,
  showPassword,
  onTogglePassword,
  disabled = false,
}: PersonalInfoSectionProps) {
  const usernameInvalid =
    form.username.length > 0 && !isUsernameValid(form.username);
  const passwordInvalid =
    form.password.length > 0 && !isPasswordValid(form.password);
  const recoveryEmailInvalid =
    form.recovery_email.length > 0 &&
    !isRecoveryEmailValid(form.recovery_email);

  return (
    <Card className="space-y-4 border-border/70 bg-card/70 p-5 backdrop-blur-sm">
      <h3 className="border-b border-border/70 pb-2 text-xs font-bold uppercase tracking-[0.2em] text-muted-foreground">
        {TEXT.AUTH.STEP_INFO}
      </h3>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label htmlFor="registration-username" className="text-xs font-semibold uppercase text-muted-foreground">{TEXT.AUTH.USERNAME}</Label>
          <Input
            id="registration-username"
            autoComplete="username"
            minLength={IDENTITY_INPUT_LIMITS.USERNAME_MIN_LENGTH}
            maxLength={IDENTITY_INPUT_LIMITS.USERNAME_MAX_LENGTH}
            pattern={usernameHTMLPattern()}
            placeholder={TEXT.AUTH.USERNAME_PLACEHOLDER}
            value={form.username}
            onChange={(event) => updateField("username", event.target.value)}
            aria-describedby="registration-username-description"
            aria-invalid={usernameInvalid}
            disabled={disabled}
            required
            className="h-11 border-border/70 bg-muted/30"
          />
          <p
            id="registration-username-description"
            className={usernameInvalid ? "text-xs text-destructive" : "text-xs text-muted-foreground"}
            aria-live="polite"
          >
            {TEXT.AUTH.USERNAME_REQUIREMENTS}
          </p>
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="registration-password" className="text-xs font-semibold uppercase text-muted-foreground">{TEXT.AUTH.PASSWORD}</Label>
          <div className="relative">
            <Input
              id="registration-password"
              type={showPassword ? "text" : "password"}
              autoComplete="new-password"
              minLength={IDENTITY_INPUT_LIMITS.PASSWORD_MIN_LENGTH}
              maxLength={IDENTITY_INPUT_LIMITS.PASSWORD_MAX_LENGTH}
              placeholder={TEXT.AUTH.PASSWORD_PLACEHOLDER}
              value={form.password}
              onChange={(event) => updateField("password", event.target.value)}
              aria-describedby="registration-password-description"
              aria-invalid={passwordInvalid}
              disabled={disabled}
              required
              className="h-11 border-border/70 bg-muted/30 pr-11"
            />
            <button
              type="button"
              aria-label={showPassword ? TEXT.AUTH.HIDE_PASSWORD : TEXT.AUTH.SHOW_PASSWORD}
              title={showPassword ? TEXT.AUTH.HIDE_PASSWORD : TEXT.AUTH.SHOW_PASSWORD}
              onClick={onTogglePassword}
              disabled={disabled}
              className="absolute right-0 top-1/2 inline-flex size-11 -translate-y-1/2 cursor-pointer items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
            >
              {showPassword ? <EyeOff aria-hidden="true" className="h-4 w-4" /> : <Eye aria-hidden="true" className="h-4 w-4" />}
            </button>
          </div>
          <p
            id="registration-password-description"
            className={passwordInvalid ? "text-xs text-destructive" : "text-xs text-muted-foreground"}
            aria-live="polite"
          >
            {TEXT.AUTH.PASSWORD_REQUIREMENTS}
          </p>
        </div>
      </div>

      <div className="space-y-1.5">
        <Label
          htmlFor="registration-recovery-email"
          className="text-xs font-semibold uppercase text-muted-foreground"
        >
          {TEXT.AUTH.RECOVERY_EMAIL}
        </Label>
        <Input
          id="registration-recovery-email"
          type="email"
          autoComplete="email"
          inputMode="email"
          maxLength={IDENTITY_INPUT_LIMITS.RECOVERY_EMAIL_MAX_LENGTH}
          placeholder={TEXT.AUTH.RECOVERY_EMAIL_PLACEHOLDER}
          value={form.recovery_email}
          onChange={(event) => updateField("recovery_email", event.target.value)}
          aria-describedby="registration-recovery-email-description"
          aria-invalid={recoveryEmailInvalid}
          disabled={disabled}
          required
          className="h-11 border-border/70 bg-muted/30"
        />
        <p
          id="registration-recovery-email-description"
          className={recoveryEmailInvalid ? "text-xs text-destructive" : "text-xs text-muted-foreground"}
          aria-live="polite"
        >
          {recoveryEmailInvalid
            ? TEXT.AUTH.RECOVERY_EMAIL_INVALID
            : TEXT.AUTH.RECOVERY_EMAIL_DESCRIPTION}
        </p>
      </div>

    </Card>
  );
}

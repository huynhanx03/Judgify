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
import { useLocale } from "@/contexts/locale-context";
import {
  isPasswordValid,
  isUsernameValid,
  usernameHTMLPattern,
} from "@/lib/auth/credentials";

interface PersonalInfoSectionProps {
  form: {
    username: string;
    password: string;
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
  const { t } = useLocale();
  const usernameInvalid =
    form.username.length > 0 && !isUsernameValid(form.username);
  const passwordInvalid =
    form.password.length > 0 && !isPasswordValid(form.password);

  return (
    <Card className="space-y-4 border-border/70 bg-surface-raised p-5">
      <h3 className="border-b border-border/70 pb-2 text-xs font-bold uppercase tracking-[0.2em] text-muted-foreground">
        {t("AUTH.STEP_INFO")}
      </h3>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label htmlFor="registration-username" className="text-xs font-semibold uppercase text-muted-foreground">{t("AUTH.USERNAME")}</Label>
          <Input
            id="registration-username"
            autoComplete="username"
            minLength={IDENTITY_INPUT_LIMITS.USERNAME_MIN_LENGTH}
            maxLength={IDENTITY_INPUT_LIMITS.USERNAME_MAX_LENGTH}
            pattern={usernameHTMLPattern()}
            placeholder={t("AUTH.USERNAME_PLACEHOLDER")}
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
            {t("AUTH.USERNAME_REQUIREMENTS")}
          </p>
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="registration-password" className="text-xs font-semibold uppercase text-muted-foreground">{t("AUTH.PASSWORD")}</Label>
          <div className="relative">
            <Input
              id="registration-password"
              type={showPassword ? "text" : "password"}
              autoComplete="new-password"
              minLength={IDENTITY_INPUT_LIMITS.PASSWORD_MIN_LENGTH}
              maxLength={IDENTITY_INPUT_LIMITS.PASSWORD_MAX_LENGTH}
              placeholder={t("AUTH.PASSWORD_PLACEHOLDER")}
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
              aria-label={showPassword ? t("AUTH.HIDE_PASSWORD") : t("AUTH.SHOW_PASSWORD")}
              title={showPassword ? t("AUTH.HIDE_PASSWORD") : t("AUTH.SHOW_PASSWORD")}
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
            {t("AUTH.PASSWORD_REQUIREMENTS")}
          </p>
        </div>
      </div>

    </Card>
  );
}

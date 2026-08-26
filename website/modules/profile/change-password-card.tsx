"use client";

import { useState, type FormEvent } from "react";
import { KeyRound, Loader2, ShieldCheck } from "lucide-react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { IDENTITY_INPUT_LIMITS } from "@/constants/identity";
import { APP_ROUTES } from "@/constants/routes";
import { TEXT } from "@/constants/text";
import { ApiError } from "@/lib/api/error";
import { authService } from "@/services/auth.service";

export function ChangePasswordCard() {
  const router = useRouter();
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [status, setStatus] = useState<"idle" | "submitting" | "success">(
    "idle",
  );

  const copy = TEXT.PROFILE.CHANGE_PASSWORD;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (status !== "idle") return;
    setError(null);

    if (
      newPassword.length < IDENTITY_INPUT_LIMITS.PASSWORD_MIN_LENGTH ||
      newPassword.length > IDENTITY_INPUT_LIMITS.PASSWORD_MAX_LENGTH
    ) {
      setError(TEXT.AUTH.PASSWORD_REQUIREMENTS);
      return;
    }
    if (newPassword !== confirmation) {
      setError(TEXT.AUTH.PASSWORD_MISMATCH);
      return;
    }
    if (newPassword === currentPassword) {
      setError(TEXT.AUTH.PASSWORD_MISMATCH);
      return;
    }

    setStatus("submitting");
    try {
      await authService.changePassword(currentPassword, newPassword);
      setCurrentPassword("");
      setNewPassword("");
      setConfirmation("");
      setStatus("success");
    } catch (caught) {
      setStatus("idle");
      setError(
        caught instanceof ApiError && caught.status === 401
          ? copy.CURRENT_PASSWORD_ERROR
          : copy.ACTION_ERROR,
      );
    }
  }

  return (
    <Card className="glass-card border-border/40">
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-lg">
          <KeyRound className="size-5 text-primary" aria-hidden="true" />
          {copy.TITLE}
        </CardTitle>
        <CardDescription>{copy.DESCRIPTION}</CardDescription>
      </CardHeader>
      <CardContent>
        {status === "success" ? (
          <div className="space-y-4">
            <div
              className="flex gap-3 rounded-xl border border-primary/25 bg-primary/10 p-4"
              role="status"
              aria-live="polite"
            >
              <ShieldCheck className="mt-0.5 size-5 shrink-0 text-primary" aria-hidden="true" />
              <div className="space-y-1 text-sm">
                <p className="font-semibold text-foreground">{copy.SUCCESS_TITLE}</p>
                <p className="leading-relaxed text-muted-foreground">
                  {copy.SUCCESS_DESCRIPTION}
                </p>
              </div>
            </div>
            <Button
              type="button"
              className="h-11 w-full"
              onClick={() => router.replace(APP_ROUTES.LOGIN)}
            >
              {copy.LOGIN_AGAIN}
            </Button>
          </div>
        ) : (
          <form className="space-y-4" onSubmit={submit} aria-busy={status === "submitting"}>
            {error ? (
              <p className="rounded-lg border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">
                {error}
              </p>
            ) : null}
            <PasswordField
              id="change-password-current"
              label={copy.CURRENT_PASSWORD}
              placeholder={copy.CURRENT_PASSWORD_PLACEHOLDER}
              value={currentPassword}
              onChange={setCurrentPassword}
              autoComplete="current-password"
              disabled={status === "submitting"}
            />
            <PasswordField
              id="change-password-new"
              label={copy.NEW_PASSWORD}
              placeholder={copy.NEW_PASSWORD_PLACEHOLDER}
              value={newPassword}
              onChange={setNewPassword}
              autoComplete="new-password"
              disabled={status === "submitting"}
            />
            <PasswordField
              id="change-password-confirmation"
              label={copy.CONFIRM_PASSWORD}
              placeholder={copy.CONFIRM_PASSWORD_PLACEHOLDER}
              value={confirmation}
              onChange={setConfirmation}
              autoComplete="new-password"
              disabled={status === "submitting"}
            />
            <Button type="submit" className="h-11 w-full" disabled={status === "submitting"}>
              {status === "submitting" ? (
                <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
              ) : null}
              {status === "submitting" ? copy.PROCESSING : copy.SUBMIT}
            </Button>
          </form>
        )}
      </CardContent>
    </Card>
  );
}

function PasswordField({
  id,
  label,
  placeholder,
  value,
  onChange,
  autoComplete,
  disabled,
}: {
  id: string;
  label: string;
  placeholder: string;
  value: string;
  onChange: (value: string) => void;
  autoComplete: string;
  disabled: boolean;
}) {
  return (
    <div className="space-y-2">
      <Label htmlFor={id}>{label}</Label>
      <Input
        id={id}
        type="password"
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        autoComplete={autoComplete}
        minLength={IDENTITY_INPUT_LIMITS.PASSWORD_MIN_LENGTH}
        maxLength={IDENTITY_INPUT_LIMITS.PASSWORD_MAX_LENGTH}
        required
        disabled={disabled}
        className="h-11 bg-muted/30"
      />
    </div>
  );
}

"use client";

import { useState, type FormEvent } from "react";
import { KeyRound, Loader2, MailPlus } from "lucide-react";
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
import { IDENTITY_INPUT_LIMITS } from "@/constants/identity";
import { TEXT } from "@/constants/text";
import {
  isRecoveryEmailValid,
  normalizeRecoveryEmail,
} from "@/lib/auth/recovery-email";

interface ReauthenticationDialogProps {
  open: boolean;
  error: string | null;
  isSubmitting: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (password: string) => Promise<void>;
}

export function ReauthenticationDialog({
  open,
  error,
  isSubmitting,
  onOpenChange,
  onSubmit,
}: ReauthenticationDialogProps) {
  const [password, setPassword] = useState("");

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!password || password.length > IDENTITY_INPUT_LIMITS.PASSWORD_MAX_LENGTH) {
      return;
    }
    await onSubmit(password);
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (isSubmitting) return;
        if (!nextOpen) setPassword("");
        onOpenChange(nextOpen);
      }}
    >
      <DialogContent className="border-border/70 bg-background/95 shadow-2xl sm:max-w-md">
        <form onSubmit={handleSubmit} className="grid gap-5">
          <DialogHeader>
            <div className="mb-1 flex size-11 items-center justify-center rounded-xl border border-primary/20 bg-primary/10 text-primary">
              <KeyRound className="size-5" aria-hidden="true" />
            </div>
            <DialogTitle className="text-xl font-bold tracking-tight">
              {TEXT.PROFILE.RECOVERY_CONTACT.REAUTH_TITLE}
            </DialogTitle>
            <DialogDescription className="leading-6">
              {TEXT.PROFILE.RECOVERY_CONTACT.REAUTH_DESCRIPTION}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-2">
            <Label htmlFor="recovery-current-password">
              {TEXT.PROFILE.RECOVERY_CONTACT.CURRENT_PASSWORD}
            </Label>
            <Input
              id="recovery-current-password"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              placeholder={TEXT.PROFILE.RECOVERY_CONTACT.CURRENT_PASSWORD_PLACEHOLDER}
              autoComplete="current-password"
              maxLength={IDENTITY_INPUT_LIMITS.PASSWORD_MAX_LENGTH}
              required
              autoFocus
              disabled={isSubmitting}
              aria-invalid={Boolean(error)}
              aria-describedby={error ? "recovery-reauth-error" : undefined}
              className="min-h-11"
            />
            {error ? (
              <p id="recovery-reauth-error" className="text-sm text-destructive" role="alert">
                {error}
              </p>
            ) : null}
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              className="min-h-11 cursor-pointer px-4"
              onClick={() => onOpenChange(false)}
              disabled={isSubmitting}
            >
              {TEXT.COMMON.CANCEL}
            </Button>
            <Button
              type="submit"
              className="min-h-11 cursor-pointer px-4"
              disabled={isSubmitting || !password}
            >
              {isSubmitting ? (
                <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
              ) : null}
              {isSubmitting
                ? TEXT.PROFILE.RECOVERY_CONTACT.PROCESSING
                : TEXT.PROFILE.RECOVERY_CONTACT.REAUTH_SUBMIT}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

interface ReplaceRecoveryContactDialogProps {
  open: boolean;
  error: string | null;
  isSubmitting: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (email: string) => Promise<void>;
}

export function ReplaceRecoveryContactDialog({
  open,
  error,
  isSubmitting,
  onOpenChange,
  onSubmit,
}: ReplaceRecoveryContactDialogProps) {
  const [email, setEmail] = useState("");
  const [fieldError, setFieldError] = useState<string | null>(null);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const normalized = normalizeRecoveryEmail(email);
    if (!isRecoveryEmailValid(normalized)) {
      setFieldError(TEXT.PROFILE.RECOVERY_CONTACT.NEW_EMAIL_INVALID);
      return;
    }
    setFieldError(null);
    await onSubmit(normalized);
  }

  function handleOpenChange(nextOpen: boolean) {
    if (isSubmitting) return;
    if (!nextOpen) {
      setEmail("");
      setFieldError(null);
    }
    onOpenChange(nextOpen);
  }

  const visibleError = fieldError ?? error;

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="border-border/70 bg-background/95 shadow-2xl sm:max-w-lg">
        <form onSubmit={handleSubmit} className="grid gap-5">
          <DialogHeader>
            <div className="mb-1 flex size-11 items-center justify-center rounded-xl border border-primary/20 bg-primary/10 text-primary">
              <MailPlus className="size-5" aria-hidden="true" />
            </div>
            <DialogTitle className="text-xl font-bold tracking-tight">
              {TEXT.PROFILE.RECOVERY_CONTACT.REPLACE_TITLE}
            </DialogTitle>
            <DialogDescription className="leading-6">
              {TEXT.PROFILE.RECOVERY_CONTACT.REPLACE_DESCRIPTION}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-2">
            <Label htmlFor="recovery-contact-email">
              {TEXT.PROFILE.RECOVERY_CONTACT.NEW_EMAIL}
            </Label>
            <Input
              id="recovery-contact-email"
              type="email"
              inputMode="email"
              value={email}
              onChange={(event) => {
                setEmail(event.target.value);
                setFieldError(null);
              }}
              placeholder={TEXT.PROFILE.RECOVERY_CONTACT.NEW_EMAIL_PLACEHOLDER}
              autoComplete="email"
              maxLength={IDENTITY_INPUT_LIMITS.RECOVERY_EMAIL_MAX_LENGTH}
              required
              autoFocus
              disabled={isSubmitting}
              aria-invalid={Boolean(visibleError)}
              aria-describedby={
                visibleError
                  ? "recovery-contact-email-help recovery-contact-email-error"
                  : "recovery-contact-email-help"
              }
              className="min-h-11"
            />
            <p id="recovery-contact-email-help" className="text-xs leading-5 text-muted-foreground">
              {TEXT.PROFILE.RECOVERY_CONTACT.NEW_EMAIL_HELP}
            </p>
            {visibleError ? (
              <p id="recovery-contact-email-error" className="text-sm text-destructive" role="alert">
                {visibleError}
              </p>
            ) : null}
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              className="min-h-11 cursor-pointer px-4"
              onClick={() => handleOpenChange(false)}
              disabled={isSubmitting}
            >
              {TEXT.COMMON.CANCEL}
            </Button>
            <Button
              type="submit"
              className="min-h-11 cursor-pointer px-4"
              disabled={isSubmitting || !email}
            >
              {isSubmitting ? (
                <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
              ) : null}
              {isSubmitting
                ? TEXT.PROFILE.RECOVERY_CONTACT.PROCESSING
                : TEXT.PROFILE.RECOVERY_CONTACT.REPLACE_SUBMIT}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

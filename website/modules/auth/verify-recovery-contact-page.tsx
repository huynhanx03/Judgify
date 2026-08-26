"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import {
  CheckCircle2,
  Loader2,
  RefreshCw,
  ShieldAlert,
  ShieldCheck,
} from "lucide-react";

import { Button, buttonVariants } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
} from "@/components/ui/card";
import { RECOVERY_CONTACT_VERIFICATION_STATUS } from "@/constants/identity";
import { APP_ROUTES } from "@/constants/routes";
import { TEXT } from "@/constants/text";
import { ApiError } from "@/lib/api/error";
import { consumeOneTimeTokenFragment } from "@/lib/auth/one-time-token";
import { authService } from "@/services/auth.service";

type VerificationState =
  | "preparing"
  | "verifying"
  | "verified"
  | "invalid"
  | "error";

export default function VerifyRecoveryContactPage() {
  const startedRef = useRef(false);
  const credentialRef = useRef<string | null>(null);
  const [state, setState] = useState<VerificationState>("preparing");

  const verifyCredential = useCallback(async (credential: string | null) => {
    if (!credential) {
      setState("invalid");
      return;
    }
    setState("verifying");
    try {
      const response = await authService.verifyRecoveryContact(credential);
      if (
        response.status !== RECOVERY_CONTACT_VERIFICATION_STATUS.VERIFIED
      ) {
        setState("error");
        return;
      }
      credentialRef.current = null;
      setState("verified");
    } catch (caught) {
      const isRejectedCredential =
        caught instanceof ApiError &&
        caught.status !== undefined &&
        caught.status >= 400 &&
        caught.status < 500 &&
        caught.status !== 429;
      if (isRejectedCredential) {
        credentialRef.current = null;
        setState("invalid");
        return;
      }
      setState("error");
    }
  }, []);

  useEffect(() => {
    if (startedRef.current) return;
    startedRef.current = true;

    const credential = consumeOneTimeTokenFragment();
    credentialRef.current = credential;
    queueMicrotask(() => void verifyCredential(credential));
  }, [verifyCredential]);

  const retry = () => {
    const credential = credentialRef.current;
    if (credential) void verifyCredential(credential);
  };

  return (
    <Card className="mx-auto w-full max-w-lg overflow-hidden border-border/70 bg-background/85 shadow-2xl backdrop-blur-xl">
      <CardHeader className="space-y-4 pb-6 text-center">
        <div className="mx-auto flex size-14 items-center justify-center rounded-2xl bg-primary/15 text-primary ring-1 ring-primary/20">
          <ShieldCheck aria-hidden="true" className="size-7" />
        </div>
        <div className="space-y-1.5">
          <h1 className="text-2xl font-bold tracking-tight">
            {TEXT.AUTH.VERIFY_RECOVERY_CONTACT_TITLE}
          </h1>
          <CardDescription className="mx-auto max-w-sm leading-relaxed">
            {TEXT.AUTH.VERIFY_RECOVERY_CONTACT_SUBTITLE}
          </CardDescription>
        </div>
      </CardHeader>

      <CardContent>
        {state === "preparing" || state === "verifying" ? (
          <div
            className="flex min-h-44 flex-col items-center justify-center gap-4 rounded-xl border border-border/70 bg-muted/35 px-6 text-center"
            role="status"
            aria-live="polite"
          >
            <Loader2
              aria-hidden="true"
              className="size-7 animate-spin text-primary motion-reduce:animate-none"
            />
            <p className="text-sm text-muted-foreground">
              {TEXT.AUTH.VERIFY_RECOVERY_CONTACT_PREPARING}
            </p>
          </div>
        ) : state === "verified" ? (
          <div className="space-y-6">
            <div
              className="flex flex-col items-center gap-3 rounded-xl border border-primary/25 bg-primary/10 px-5 py-6 text-center"
              role="status"
              aria-live="polite"
            >
              <CheckCircle2 aria-hidden="true" className="size-8 text-primary" />
              <div className="space-y-1.5">
                <p className="font-semibold text-foreground">
                  {TEXT.AUTH.VERIFY_RECOVERY_CONTACT_SUCCESS_TITLE}
                </p>
                <p className="text-sm leading-relaxed text-muted-foreground">
                  {TEXT.AUTH.VERIFY_RECOVERY_CONTACT_SUCCESS_DESCRIPTION}
                </p>
              </div>
            </div>
            <Link
              href={APP_ROUTES.LOGIN}
              className={buttonVariants({
                className: "h-11 w-full font-semibold",
              })}
            >
              {TEXT.AUTH.VERIFY_RECOVERY_CONTACT_LOGIN}
            </Link>
          </div>
        ) : state === "invalid" ? (
          <div className="space-y-6">
            <div
              className="flex items-start gap-3 rounded-xl border border-destructive/25 bg-destructive/10 px-4 py-4 text-sm text-destructive"
              role="alert"
            >
              <ShieldAlert aria-hidden="true" className="mt-0.5 size-5 shrink-0" />
              <p className="leading-relaxed">
                {TEXT.AUTH.VERIFY_RECOVERY_CONTACT_INVALID}
              </p>
            </div>
            <Link
              href={APP_ROUTES.LOGIN}
              className={buttonVariants({
                variant: "outline",
                className: "h-11 w-full",
              })}
            >
              {TEXT.AUTH.BACK_TO_LOGIN}
            </Link>
          </div>
        ) : (
          <div className="space-y-6">
            <div
              className="flex items-start gap-3 rounded-xl border border-border bg-muted/40 px-4 py-4 text-sm text-foreground"
              role="alert"
            >
              <ShieldAlert aria-hidden="true" className="mt-0.5 size-5 shrink-0 text-primary" />
              <p className="leading-relaxed">
                {TEXT.AUTH.VERIFY_RECOVERY_CONTACT_FAILED}
              </p>
            </div>
            <Button
              type="button"
              onClick={retry}
              className="h-11 w-full font-semibold"
            >
              <RefreshCw aria-hidden="true" className="size-4" />
              {TEXT.AUTH.VERIFY_RECOVERY_CONTACT_RETRY}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

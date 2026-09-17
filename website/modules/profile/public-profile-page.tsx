"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import {
  ArrowLeft,
  AtSign,
  CalendarDays,
  CircleAlert,
  Eye,
  EyeOff,
  RefreshCw,
  ShieldCheck,
} from "lucide-react";

import { LoadingSpinner } from "@/components/loading-spinner";
import { PublicPageLead } from "@/components/public-page-lead";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Button, buttonVariants } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { APP_ROUTES } from "@/constants/routes";
import { formatProfileAttributeNumber } from "@/constants/profile-attributes";
import { TEXT } from "@/constants/text";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { ApiError } from "@/lib/api/error";
import { isUsernameValid } from "@/lib/auth/credentials";
import { formatCalendarDate, formatDateTimeMinute } from "@/lib/format";
import { publicProfileService } from "@/services/public-profile.service";
import type {
  PublicProfile,
  PublicProfileAttribute,
} from "@/types/public-profile";

function publicAttributeValue(attribute: PublicProfileAttribute): string {
  if (attribute.data_type === "boolean") {
    return attribute.value
      ? TEXT.PUBLIC_PROFILE.BOOLEAN_TRUE
      : TEXT.PUBLIC_PROFILE.BOOLEAN_FALSE;
  }
  if (attribute.data_type === "number") {
    return formatProfileAttributeNumber(attribute.value as number);
  }
  if (attribute.data_type === "date") {
    return formatCalendarDate(attribute.value as string);
  }
  return attribute.value as string;
}

function profileInitials(username: string): string {
  return username.slice(0, 2).toUpperCase();
}

export default function PublicProfilePage() {
  const params = useParams();
  const requestedUsername =
    typeof params.username === "string" ? params.username : "";
  const validUsername = isUsernameValid(requestedUsername);
  const profileResource = useRetryableResource<PublicProfile | null>({
    resetKey: requestedUsername || "invalid-public-profile",
    enabled: validUsername,
    initialData: null,
    load: (signal) => publicProfileService.get(requestedUsername, signal),
  });
  const notFound =
    !validUsername ||
    (profileResource.error instanceof ApiError &&
      profileResource.error.status === 404);

  if (validUsername && profileResource.status === "loading") {
    return <LoadingSpinner label={TEXT.PUBLIC_PROFILE.LOADING} />;
  }

  if (notFound) {
    return <PublicProfileUnavailable />;
  }

  if (profileResource.status === "error" || !profileResource.data) {
    return (
      <PublicProfileFeedback
        title={TEXT.PUBLIC_PROFILE.LOAD_ERROR_TITLE}
        description={TEXT.PUBLIC_PROFILE.LOAD_ERROR_DESCRIPTION}
        actionLabel={TEXT.PUBLIC_PROFILE.RETRY}
        onAction={profileResource.retry}
      />
    );
  }

  const profile = profileResource.data;

  return (
    <div className="space-y-10 pb-16 motion-safe:animate-in motion-safe:fade-in sm:space-y-12 sm:pb-20">
      <PublicPageLead
        eyebrow={TEXT.PUBLIC_PROFILE.EYEBROW}
        title={profile.username}
        actions={
          <Link
            href={APP_ROUTES.ARENA}
            className={buttonVariants({ variant: "outline", size: "sm" })}
          >
            <ArrowLeft className="size-4" aria-hidden="true" />
            {TEXT.PUBLIC_PROFILE.BACK_TO_ARENA}
          </Link>
        }
      />

      <Card className="border-border/60 bg-card">
        <CardContent className="grid gap-5 p-5 sm:grid-cols-[auto_1fr] sm:items-center sm:p-8">
          <Avatar className="size-20 border border-border sm:size-24">
            <AvatarFallback className="bg-primary/10 text-3xl font-bold text-primary">
              {profileInitials(profile.username)}
            </AvatarFallback>
          </Avatar>
          <div className="min-w-0 space-y-3 pb-1">
            <p className="flex items-center gap-1.5 text-sm text-muted-foreground">
              <AtSign className="size-4" aria-hidden="true" />
              {profile.username}
            </p>
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <CalendarDays
                className="size-4 text-primary"
                aria-hidden="true"
              />
              <span>{TEXT.PUBLIC_PROFILE.JOINED}</span>
              <time
                className="font-semibold text-foreground"
                dateTime={profile.joined_at}
              >
                {formatDateTimeMinute(profile.joined_at)}
              </time>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card className="border-border/60 bg-card">
        <CardHeader className="gap-3 border-b border-border/60 bg-muted/15 sm:flex-row sm:items-start">
          <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
            <ShieldCheck className="size-5" aria-hidden="true" />
          </div>
          <div>
            <CardTitle className="text-lg">
              {TEXT.PUBLIC_PROFILE.ATTRIBUTES_TITLE}
            </CardTitle>
            <CardDescription className="mt-1 leading-5">
              {TEXT.PUBLIC_PROFILE.ATTRIBUTES_DESCRIPTION}
            </CardDescription>
          </div>
        </CardHeader>
        <CardContent className="p-4 sm:p-6">
          {profile.attributes.length === 0 ? (
            <div className="flex min-h-48 flex-col items-center justify-center rounded-2xl border border-dashed border-border/80 bg-muted/15 px-6 text-center">
              <Eye
                className="size-6 text-muted-foreground"
                aria-hidden="true"
              />
              <h2 className="mt-3 font-semibold">
                {TEXT.PUBLIC_PROFILE.EMPTY_TITLE}
              </h2>
              <p className="mt-1 max-w-md text-sm leading-5 text-muted-foreground">
                {TEXT.PUBLIC_PROFILE.EMPTY_DESCRIPTION}
              </p>
            </div>
          ) : (
            <dl className="grid gap-3 sm:grid-cols-2">
              {profile.attributes.map((attribute) => (
                <div
                  key={attribute.key}
                  className="rounded-2xl border border-border/70 bg-background/70 p-4 transition-colors duration-200 hover:border-primary/30 hover:bg-muted/20 motion-reduce:transition-none"
                >
                  <dt className="font-semibold text-foreground">
                    {attribute.label}
                  </dt>
                  <dd>
                    {attribute.description ? (
                      <p className="mt-1 text-xs leading-5 text-muted-foreground">
                        {attribute.description}
                      </p>
                    ) : null}
                    <p className="mt-3 break-words text-base font-semibold text-primary">
                      {publicAttributeValue(attribute)}
                    </p>
                  </dd>
                </div>
              ))}
            </dl>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function PublicProfileUnavailable() {
  return (
    <div className="mx-auto flex min-h-[60vh] max-w-xl items-center px-4">
      <Card className="w-full border-border/60 bg-card/85 shadow-lg">
        <CardContent className="flex flex-col items-center px-6 py-12 text-center">
          <div className="flex size-14 items-center justify-center rounded-2xl bg-muted text-muted-foreground">
            <EyeOff className="size-6" aria-hidden="true" />
          </div>
          <h1 className="mt-5 text-xl font-bold tracking-tight">
            {TEXT.PUBLIC_PROFILE.NOT_FOUND_TITLE}
          </h1>
          <p className="mt-2 max-w-md text-sm leading-6 text-muted-foreground">
            {TEXT.PUBLIC_PROFILE.NOT_FOUND_DESCRIPTION}
          </p>
          <Link
            href={APP_ROUTES.ARENA}
            className={buttonVariants({
              variant: "outline",
              className: "mt-6",
            })}
          >
            <ArrowLeft className="size-4" aria-hidden="true" />
            {TEXT.PUBLIC_PROFILE.BACK_TO_ARENA}
          </Link>
        </CardContent>
      </Card>
    </div>
  );
}

function PublicProfileFeedback({
  title,
  description,
  actionLabel,
  onAction,
}: {
  title: string;
  description: string;
  actionLabel: string;
  onAction: () => void;
}) {
  return (
    <div className="mx-auto flex min-h-[60vh] max-w-xl items-center px-4">
      <Card className="w-full border-destructive/20 bg-card/85 shadow-lg">
        <CardContent
          className="flex flex-col items-center px-6 py-12 text-center"
          role="alert"
        >
          <div className="flex size-14 items-center justify-center rounded-2xl bg-destructive/10 text-destructive">
            <CircleAlert className="size-6" aria-hidden="true" />
          </div>
          <h1 className="mt-5 text-xl font-bold tracking-tight">{title}</h1>
          <p className="mt-2 max-w-md text-sm leading-6 text-muted-foreground">
            {description}
          </p>
          <Button
            type="button"
            variant="outline"
            className="mt-6"
            onClick={onAction}
          >
            <RefreshCw className="size-4" aria-hidden="true" />
            {actionLabel}
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}

"use client";

import {
  Bone,
  CircleAlert,
  Loader2,
  RefreshCw,
  Sparkles,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Separator } from "@/components/ui/separator";
import { TEXT } from "@/constants/text";
import type { AsyncResourceStatus } from "@/hooks/use-retryable-resource";
import { TraitCard } from "@/modules/cultivation/trait-card";
import type { TraitResponse } from "@/types/cultivation";

interface TraitCodexModalProps {
  open: boolean;
  onClose: () => void;
  traits: TraitResponse[];
  status: AsyncResourceStatus;
  onRetry: () => void;
}

function TraitGroup({
  title,
  empty,
  traits,
  icon,
}: {
  title: string;
  empty: string;
  traits: TraitResponse[];
  icon: "bone" | "talent";
}) {
  const Icon = icon === "bone" ? Bone : Sparkles;
  return (
    <section aria-labelledby={`trait-group-${icon}`}>
      <h3
        id={`trait-group-${icon}`}
        className="mb-3 flex items-center gap-2 text-xs font-bold tracking-wider text-foreground uppercase"
      >
        <Icon className="size-4 text-primary" aria-hidden="true" />
        {title} ({traits.length})
      </h3>
      {traits.length > 0 ? (
        <div className="grid grid-cols-1 gap-2 md:grid-cols-2 lg:grid-cols-3">
          {traits.map((trait) => (
            <TraitCard key={trait.id} trait={trait} variant="compact" />
          ))}
        </div>
      ) : (
        <p className="rounded-xl border border-dashed border-border bg-muted/15 px-4 py-8 text-center text-sm text-muted-foreground">
          {empty}
        </p>
      )}
    </section>
  );
}

export function TraitCodexModal({
  open,
  onClose,
  traits,
  status,
  onRetry,
}: TraitCodexModalProps) {
  const rootBones = traits.filter((trait) => trait.type === "root_bone");
  const talents = traits.filter((trait) => trait.type === "talent");
  const initialLoading =
    (status === "idle" || status === "loading") &&
    traits.length === 0;
  const unavailable = status === "error" && traits.length === 0;
  const stale = status === "error" && traits.length > 0;

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen) onClose();
      }}
    >
      <DialogContent className="grid max-h-[88dvh] grid-rows-[auto_minmax(0,1fr)] gap-0 overflow-hidden p-0 sm:max-w-6xl">
        <DialogHeader className="border-b border-border px-6 py-5 pr-14">
          <DialogTitle className="text-xl font-bold tracking-wide text-primary">
            {TEXT.CULTIVATION.CODEX_TITLE}
          </DialogTitle>
          <DialogDescription>
            {TEXT.CULTIVATION.CODEX_DESCRIPTION}
          </DialogDescription>
        </DialogHeader>

        <div className="min-h-0 overflow-y-auto px-6 py-6">
          {initialLoading ? (
            <div
              className="flex min-h-[50dvh] items-center justify-center gap-3 text-sm text-muted-foreground"
              role="status"
            >
              <Loader2
                className="size-6 animate-spin text-primary motion-reduce:animate-none"
                aria-hidden="true"
              />
              {TEXT.CULTIVATION.CODEX_LOADING}
            </div>
          ) : unavailable ? (
            <div
              className="flex min-h-[50dvh] flex-col items-center justify-center px-6 text-center"
              role="alert"
            >
              <div className="flex size-12 items-center justify-center rounded-2xl bg-destructive/10 text-destructive">
                <CircleAlert className="size-5" aria-hidden="true" />
              </div>
              <p className="mt-4 font-semibold text-foreground">
                {TEXT.CULTIVATION.CODEX_LOAD_ERROR}
              </p>
              <p className="mt-2 max-w-md text-sm leading-6 text-muted-foreground">
                {TEXT.CULTIVATION.CODEX_LOAD_ERROR_DESCRIPTION}
              </p>
              <Button
                type="button"
                variant="outline"
                className="mt-5"
                onClick={onRetry}
              >
                <RefreshCw className="size-4" aria-hidden="true" />
                {TEXT.COMMON.RETRY}
              </Button>
            </div>
          ) : (
            <div className="space-y-6">
              {stale ? (
                <div
                  className="flex flex-wrap items-center gap-3 rounded-xl border border-warning/25 bg-warning/5 p-3 text-sm text-muted-foreground"
                  role="status"
                >
                  <CircleAlert
                    className="size-4 shrink-0 text-warning"
                    aria-hidden="true"
                  />
                  <span className="min-w-0 flex-1">
                    {TEXT.CULTIVATION.CODEX_STALE}
                  </span>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={onRetry}
                  >
                    <RefreshCw className="size-4" aria-hidden="true" />
                    {TEXT.COMMON.RETRY}
                  </Button>
                </div>
              ) : null}
              <TraitGroup
                title={TEXT.AUTH.TRAIT_ROOT_BONE}
                empty={TEXT.CULTIVATION.CODEX_ROOT_EMPTY}
                traits={rootBones}
                icon="bone"
              />
              <Separator />
              <TraitGroup
                title={TEXT.AUTH.TRAIT_TALENT}
                empty={TEXT.CULTIVATION.CODEX_TALENT_EMPTY}
                traits={talents}
                icon="talent"
              />
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}

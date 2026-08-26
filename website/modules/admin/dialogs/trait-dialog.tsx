"use client";

import { useState } from "react";
import { Archive, Loader2 } from "lucide-react";
import { ConfirmDialog } from "@/components/confirm-dialog";
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
import { Textarea } from "@/components/ui/textarea";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { notify } from "@/lib/toast";
import type {
  TraitMutationInput,
  TraitUpdateInput,
} from "@/services/cultivation.service";
import type {
  RarityResponse,
  TraitEffectScope,
  TraitEffectSpec,
  TraitResponse,
  TraitType,
} from "@/types/cultivation";

interface TraitDialogProps {
  open: boolean;
  editing: TraitResponse | null;
  onSave: (input: TraitMutationInput | TraitUpdateInput) => Promise<void>;
  onArchive?: (expectedVersion: number, reason: string) => Promise<void>;
  onClose: () => void;
  isSaving: boolean;
  rarities: RarityResponse[];
}

type EffectKind = TraitEffectSpec["kind"];

function effectValue(effect: TraitEffectSpec | undefined): string {
  if (!effect) return "0";
  return String(
    effect.kind === "exp_multiplier"
      ? effect.multiplier_delta_bps
      : effect.flat_bonus,
  );
}

function effectElements(effect: TraitEffectSpec | undefined): string {
  return effect?.element_codes?.join(", ") ?? "";
}

function parseElementCodes(value: string): string[] {
  return [...new Set(
    value
      .split(",")
      .map((code) => code.trim().toLowerCase())
      .filter(Boolean),
  )].sort();
}

export function TraitDialog({
  open,
  editing,
  onSave,
  onArchive,
  onClose,
  isSaving,
  rarities,
}: TraitDialogProps) {
  const currentEffect = editing?.effect_revision?.effect;
  const [type, setType] = useState<TraitType>(editing?.type ?? "root_bone");
  const [code, setCode] = useState(editing?.code ?? "");
  const [name, setName] = useState(editing?.name ?? "");
  const [rarityID, setRarityID] = useState(editing?.rarity?.id ?? "");
  const [description, setDescription] = useState(editing?.description ?? "");
  const [displayOrder, setDisplayOrder] = useState(
    String(editing?.display_order ?? 0),
  );
  const [active, setActive] = useState(editing?.active ?? true);
  const [effectKind, setEffectKind] = useState<EffectKind>(
    currentEffect?.kind ?? "exp_multiplier",
  );
  const [effectScope, setEffectScope] = useState<TraitEffectScope>(
    currentEffect?.scope ?? "all",
  );
  const [value, setValue] = useState(effectValue(currentEffect));
  const [elementCodes, setElementCodes] = useState(effectElements(currentEffect));
  const [reason, setReason] = useState("");
  const [archiveOpen, setArchiveOpen] = useState(false);
  const [isArchiving, setIsArchiving] = useState(false);

  const parsedDisplayOrder = Number(displayOrder);
  const parsedValue = Number(value);
  const targets = parseElementCodes(elementCodes);
  const codeValid = /^[a-z][a-z0-9_]{0,63}$/.test(code.trim());
  const reasonValid = reason.trim().length >= 3 && reason.trim().length <= 1024;
  const valueValid =
    Number.isSafeInteger(parsedValue) &&
    (effectKind === "exp_multiplier"
      ? parsedValue >= -10_000 && parsedValue <= 90_000
      : parsedValue >= -1_000_000_000 && parsedValue <= 1_000_000_000);
  const targetValid =
    effectScope === "all" ||
    (targets.length >= 1 &&
      targets.length <= 16 &&
      targets.every((target) => /^[a-z][a-z0-9_]{0,63}$/.test(target)));
  const formValid = Boolean(
    name.trim() &&
      rarityID &&
      (editing || codeValid) &&
      Number.isSafeInteger(parsedDisplayOrder) &&
      parsedDisplayOrder >= 0 &&
      parsedDisplayOrder <= 1_000_000 &&
      valueValid &&
      targetValid &&
      reasonValid,
  );
  const busy = isSaving || isArchiving;

  function buildEffect(): TraitEffectSpec {
    const scope = effectScope;
    const target = scope === "element" ? { element_codes: targets } : {};
    return effectKind === "exp_multiplier"
      ? {
          v: 1,
          kind: "exp_multiplier",
          scope,
          ...target,
          multiplier_delta_bps: parsedValue,
        }
      : {
          v: 1,
          kind: "exp_bonus",
          scope,
          ...target,
          flat_bonus: parsedValue,
        };
  }

  function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!formValid) return;
    const shared = {
      type,
      name: name.trim(),
      rarity_id: rarityID,
      description: description.trim() || undefined,
      display_order: parsedDisplayOrder,
      effect: buildEffect(),
      reason: reason.trim(),
    };
    if (editing) {
      void onSave({
        ...shared,
        active,
        expected_version: editing.version,
      });
      return;
    }
    void onSave({ ...shared, code: code.trim() });
  }

  async function archiveTrait() {
    if (!editing || !onArchive || !reasonValid || isArchiving) return;
    setIsArchiving(true);
    try {
      await onArchive(editing.version, reason.trim());
      setArchiveOpen(false);
    } catch {
      notify.error(ADMIN_TEXT.TRAITS.ARCHIVE_ERROR);
    } finally {
      setIsArchiving(false);
    }
  }

  return (
    <>
      <Dialog open={open} onOpenChange={(next) => { if (!next) onClose(); }}>
        <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>
              {editing
                ? ADMIN_TEXT.TRAITS.DIALOG_EDIT_TITLE
                : ADMIN_TEXT.TRAITS.DIALOG_CREATE_TITLE}
            </DialogTitle>
            <DialogDescription>
              {editing
                ? ADMIN_TEXT.TRAITS.DIALOG_EDIT_DESC
                : ADMIN_TEXT.TRAITS.DIALOG_CREATE_DESC}
            </DialogDescription>
          </DialogHeader>

          <form className="space-y-6" onSubmit={submit}>
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="trait-code">{ADMIN_TEXT.TRAITS.CODE_LABEL}</Label>
                <Input
                  id="trait-code"
                  value={code}
                  onChange={(event) => setCode(event.target.value.toLowerCase())}
                  placeholder={ADMIN_TEXT.TRAITS.CODE_PLACEHOLDER}
                  disabled={Boolean(editing) || busy}
                  aria-invalid={!editing && code.length > 0 && !codeValid}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="trait-type">{ADMIN_TEXT.FIELDS.TYPE}</Label>
                <select
                  id="trait-type"
                  value={type}
                  onChange={(event) => setType(event.target.value as TraitType)}
                  disabled={busy}
                  className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                >
                  <option value="root_bone">{ADMIN_TEXT.FIELDS.TYPE_ROOT_BONE}</option>
                  <option value="talent">{ADMIN_TEXT.FIELDS.TYPE_TALENT}</option>
                </select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="trait-name">{ADMIN_TEXT.FIELDS.NAME}</Label>
                <Input
                  id="trait-name"
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  placeholder={ADMIN_TEXT.EXAMPLES.TRAIT}
                  maxLength={100}
                  disabled={busy}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="trait-rarity">{ADMIN_TEXT.FIELDS.RARITY}</Label>
                <select
                  id="trait-rarity"
                  value={rarityID}
                  onChange={(event) => setRarityID(event.target.value)}
                  disabled={busy}
                  className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                >
                  <option value="">{ADMIN_TEXT.FIELDS.SELECT_RARITY_PLACEHOLDER}</option>
                  {rarities.map((rarity) => (
                    <option key={rarity.id} value={rarity.id}>{rarity.name}</option>
                  ))}
                </select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="trait-display-order">
                  {ADMIN_TEXT.TRAITS.DISPLAY_ORDER}
                </Label>
                <Input
                  id="trait-display-order"
                  type="number"
                  min={0}
                  max={1_000_000}
                  step={1}
                  value={displayOrder}
                  onChange={(event) => setDisplayOrder(event.target.value)}
                  disabled={busy}
                />
              </div>
              {editing ? (
                <div className="space-y-2">
                  <Label htmlFor="trait-active">{ADMIN_TEXT.TRAITS.COL_STATUS}</Label>
                  <select
                    id="trait-active"
                    value={active ? "active" : "archived"}
                    onChange={(event) => setActive(event.target.value === "active")}
                    disabled={busy}
                    className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                  >
                    <option value="active">{ADMIN_TEXT.TRAITS.ACTIVE}</option>
                    <option value="archived">{ADMIN_TEXT.TRAITS.ARCHIVED}</option>
                  </select>
                </div>
              ) : null}
            </div>

            <div className="space-y-2">
              <Label htmlFor="trait-description">{ADMIN_TEXT.FIELDS.DESCRIPTION}</Label>
              <Textarea
                id="trait-description"
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                placeholder={ADMIN_TEXT.FIELDS.DESCRIPTION_PLACEHOLDER}
                maxLength={500}
                disabled={busy}
              />
            </div>

            <fieldset className="space-y-4 rounded-xl border border-border bg-muted/15 p-4">
              <legend className="px-1 text-sm font-bold">
                {ADMIN_TEXT.TRAITS.COL_EFFECT}
              </legend>
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="trait-effect-kind">{ADMIN_TEXT.TRAITS.EFFECT_KIND}</Label>
                  <select
                    id="trait-effect-kind"
                    value={effectKind}
                    onChange={(event) => setEffectKind(event.target.value as EffectKind)}
                    disabled={busy}
                    className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                  >
                    <option value="exp_multiplier">{ADMIN_TEXT.TRAITS.EFFECT_MULTIPLIER}</option>
                    <option value="exp_bonus">{ADMIN_TEXT.TRAITS.EFFECT_BONUS}</option>
                  </select>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="trait-effect-scope">{ADMIN_TEXT.TRAITS.EFFECT_SCOPE}</Label>
                  <select
                    id="trait-effect-scope"
                    value={effectScope}
                    onChange={(event) => setEffectScope(event.target.value as TraitEffectScope)}
                    disabled={busy}
                    className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                  >
                    <option value="all">{ADMIN_TEXT.TRAITS.EFFECT_SCOPE_ALL}</option>
                    <option value="element">{ADMIN_TEXT.TRAITS.EFFECT_SCOPE_ELEMENT}</option>
                  </select>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="trait-effect-value">
                    {effectKind === "exp_multiplier"
                      ? ADMIN_TEXT.TRAITS.EFFECT_VALUE_BPS
                      : ADMIN_TEXT.TRAITS.EFFECT_VALUE_BONUS}
                  </Label>
                  <Input
                    id="trait-effect-value"
                    type="number"
                    step={1}
                    value={value}
                    onChange={(event) => setValue(event.target.value)}
                    disabled={busy}
                    aria-invalid={value.length > 0 && !valueValid}
                  />
                </div>
                {effectScope === "element" ? (
                  <div className="space-y-2">
                    <Label htmlFor="trait-effect-elements">
                      {ADMIN_TEXT.TRAITS.EFFECT_ELEMENTS}
                    </Label>
                    <Input
                      id="trait-effect-elements"
                      value={elementCodes}
                      onChange={(event) => setElementCodes(event.target.value)}
                      placeholder={ADMIN_TEXT.TRAITS.EFFECT_ELEMENTS_PLACEHOLDER}
                      disabled={busy}
                      aria-describedby="trait-effect-elements-help"
                      aria-invalid={elementCodes.length > 0 && !targetValid}
                    />
                    <p id="trait-effect-elements-help" className="text-xs text-muted-foreground">
                      {ADMIN_TEXT.TRAITS.EFFECT_ELEMENTS_HELP}
                    </p>
                  </div>
                ) : null}
              </div>
            </fieldset>

            <div className="space-y-2">
              <Label htmlFor="trait-reason">{ADMIN_TEXT.TRAITS.REASON_LABEL}</Label>
              <Textarea
                id="trait-reason"
                value={reason}
                onChange={(event) => setReason(event.target.value)}
                placeholder={ADMIN_TEXT.TRAITS.REASON_PLACEHOLDER}
                maxLength={1024}
                disabled={busy}
                aria-invalid={reason.length > 0 && !reasonValid}
              />
            </div>

            <DialogFooter className="gap-2 sm:justify-between">
              <div>
                {editing?.active && onArchive ? (
                  <Button
                    type="button"
                    variant="destructive"
                    onClick={() => setArchiveOpen(true)}
                    disabled={!reasonValid || busy}
                  >
                    <Archive className="size-4" aria-hidden="true" />
                    {ADMIN_TEXT.TRAITS.ARCHIVE_ACTION}
                  </Button>
                ) : null}
              </div>
              <div className="flex flex-col-reverse gap-2 sm:flex-row">
                <Button type="button" variant="outline" onClick={onClose} disabled={busy}>
                  {TEXT.COMMON.CANCEL}
                </Button>
                <Button type="submit" disabled={!formValid || busy}>
                  {isSaving ? (
                    <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
                  ) : null}
                  {editing ? TEXT.COMMON.SAVE : TEXT.COMMON.CREATE}
                </Button>
              </div>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={archiveOpen}
        onOpenChange={setArchiveOpen}
        onConfirm={() => void archiveTrait()}
        title={ADMIN_TEXT.TRAITS.ARCHIVE_TITLE}
        description={ADMIN_TEXT.TRAITS.ARCHIVE_DESCRIPTION}
        confirmLabel={ADMIN_TEXT.TRAITS.ARCHIVE_ACTION}
        loading={isArchiving}
        confirmDisabled={!reasonValid}
      />
    </>
  );
}

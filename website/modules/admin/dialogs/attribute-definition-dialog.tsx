"use client";

import { useState } from "react";
import { Loader2 } from "lucide-react";
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
import {
  PROFILE_ATTRIBUTE_KEY_PATTERN,
  PROFILE_ATTRIBUTE_LIMITS,
} from "@/constants/profile-attributes";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { attributeDataTypeLabel } from "@/lib/admin/auxiliary-form";
import {
  isCanonicalProfileValidationText,
  parseProfileAttributeValidation,
} from "@/lib/profile/profile-validation-schema";
import {
  ATTRIBUTE_DATA_TYPES,
  type AttributeDataType,
  type AttributeDefinition,
  type AttributeVisibility,
  type CreateAttributeDefinitionInput,
  type UpdateAttributeDefinitionInput,
} from "@/types/admin-auxiliary";

interface AttributeDefinitionDialogProps {
  open: boolean;
  editing: AttributeDefinition | null;
  onCreate: (input: CreateAttributeDefinitionInput) => Promise<void>;
  onUpdate: (input: UpdateAttributeDefinitionInput) => Promise<void>;
  onClose: () => void;
  isSaving: boolean;
}

export function AttributeDefinitionDialog({
  open,
  editing,
  onCreate,
  onUpdate,
  onClose,
  isSaving,
}: AttributeDefinitionDialogProps) {
  const [key, setKey] = useState(editing?.key ?? "");
  const [dataType, setDataType] = useState<AttributeDataType>(
    editing?.draft_revision.data_type ?? "string",
  );
  const [displayOrder, setDisplayOrder] = useState(
    String(editing?.draft_revision?.display_order ?? 0),
  );
  const [label, setLabel] = useState(editing?.draft_revision?.label ?? editing?.key ?? "");
  const [description, setDescription] = useState(
    editing?.draft_revision.description ?? "",
  );
  const [visibility, setVisibility] = useState<AttributeVisibility>(editing?.draft_revision?.visibility ?? "private");
  const [userEditable, setUserEditable] = useState(editing?.draft_revision?.user_editable ?? false);
  const [requiredOnOnboarding, setRequiredOnOnboarding] = useState(editing?.required_on_onboarding ?? false);
  const [validation, setValidation] = useState(() => JSON.stringify(editing?.draft_revision?.validation ?? {}, null, 2));
  const [reason, setReason] = useState("");
  const normalizedKey = key.trim();
  const normalizedDescription = description.trim();
  const keyInvalid = !PROFILE_ATTRIBUTE_KEY_PATTERN.test(normalizedKey);
  const descriptionInvalid =
    normalizedDescription.length >
    PROFILE_ATTRIBUTE_LIMITS.DESCRIPTION_MAX_LENGTH;
  const normalizedLabel = label.trim();
  const normalizedReason = reason.trim();
  const parsedDisplayOrder = Number(displayOrder);
  const displayOrderInvalid =
    !/^\d+$/.test(displayOrder) ||
    !Number.isSafeInteger(parsedDisplayOrder) ||
    parsedDisplayOrder < PROFILE_ATTRIBUTE_LIMITS.DISPLAY_ORDER_MIN ||
    parsedDisplayOrder > PROFILE_ATTRIBUTE_LIMITS.DISPLAY_ORDER_MAX;
  let parsedValidation: Record<string, unknown> | null = null;
  try {
    const parsed: unknown = JSON.parse(validation);
    const candidate = parseProfileAttributeValidation(
      parsed,
      dataType,
      "$.validation",
    );
    if (isCanonicalProfileValidationText(validation, parsed)) {
      parsedValidation = candidate;
    }
  } catch {
    parsedValidation = null;
  }
  const revisionInvalid =
    normalizedLabel.length < 2 ||
    normalizedLabel.length > 128 ||
    normalizedReason.length < 3 ||
    normalizedReason.length > 500 ||
    displayOrderInvalid ||
    parsedValidation === null;

  function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (descriptionInvalid || revisionInvalid || (!editing && keyInvalid) || !parsedValidation) return;
    if (editing) {
      void onUpdate({
        expected_version: editing.version,
        display_order: parsedDisplayOrder,
        data_type: dataType,
        label: normalizedLabel,
        description: normalizedDescription,
        visibility,
        user_editable: userEditable,
        required_on_onboarding: requiredOnOnboarding,
        validation: parsedValidation,
        reason: normalizedReason,
      });
      return;
    }
    void onCreate({
      key: normalizedKey,
      display_order: parsedDisplayOrder,
      data_type: dataType,
      label: normalizedLabel,
      description: normalizedDescription,
      visibility,
      user_editable: userEditable,
      required_on_onboarding: requiredOnOnboarding,
      validation: parsedValidation,
      reason: normalizedReason,
    });
  }

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => { if (!nextOpen) onClose(); }}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-md">
        <DialogHeader className="min-w-0 pr-7">
          <DialogTitle className="break-words leading-snug">{editing ? ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.DIALOG_EDIT_TITLE : ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.DIALOG_CREATE_TITLE}</DialogTitle>
          <DialogDescription className="break-words">{editing ? ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.DIALOG_EDIT_DESC : ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.DIALOG_CREATE_DESC}</DialogDescription>
        </DialogHeader>
        <form className="space-y-4 py-2" onSubmit={handleSubmit} aria-busy={isSaving}>
          <div className="space-y-2">
            <Label htmlFor="attribute-definition-key">{ADMIN_TEXT.FIELDS.KEY}</Label>
            <Input
              id="attribute-definition-key"
              value={key}
              onChange={(event) => setKey(event.target.value)}
              placeholder={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.KEY_PLACEHOLDER}
              minLength={PROFILE_ATTRIBUTE_LIMITS.KEY_MIN_LENGTH}
              maxLength={PROFILE_ATTRIBUTE_LIMITS.KEY_MAX_LENGTH}
              pattern={PROFILE_ATTRIBUTE_KEY_PATTERN.source}
              readOnly={Boolean(editing)}
              aria-readonly={Boolean(editing)}
              disabled={isSaving}
              aria-invalid={!editing && keyInvalid}
              aria-describedby={
                editing
                  ? "attribute-definition-key-immutable"
                  : keyInvalid
                    ? "attribute-definition-key-error"
                    : undefined
              }
            />
            {editing ? (
              <p
                id="attribute-definition-key-immutable"
                className="text-sm text-muted-foreground"
              >
                {ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.KEY_IMMUTABLE}
              </p>
            ) : null}
            {!editing && keyInvalid ? <p id="attribute-definition-key-error" className="text-sm text-destructive">{ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.KEY_INVALID}</p> : null}
          </div>
          <div className="space-y-2">
            <Label htmlFor="attribute-definition-label">{ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.LABEL}</Label>
            <Input
              id="attribute-definition-label"
              value={label}
              onChange={(event) => setLabel(event.target.value)}
              placeholder={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.LABEL_PLACEHOLDER}
              minLength={PROFILE_ATTRIBUTE_LIMITS.LABEL_MIN_LENGTH}
              maxLength={PROFILE_ATTRIBUTE_LIMITS.LABEL_MAX_LENGTH}
              disabled={isSaving}
            />
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="attribute-definition-data-type">{ADMIN_TEXT.FIELDS.DATA_TYPE}</Label>
              <select
                id="attribute-definition-data-type"
                className="flex min-h-11 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                value={dataType}
                onChange={(event) => setDataType(event.target.value as AttributeDataType)}
                disabled={isSaving}
              >
                {ATTRIBUTE_DATA_TYPES.map((value) => <option key={value} value={value}>{attributeDataTypeLabel(value)}</option>)}
              </select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="attribute-definition-display-order">
                {ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.DISPLAY_ORDER}
              </Label>
              <Input
                id="attribute-definition-display-order"
                type="number"
                inputMode="numeric"
                min={PROFILE_ATTRIBUTE_LIMITS.DISPLAY_ORDER_MIN}
                max={PROFILE_ATTRIBUTE_LIMITS.DISPLAY_ORDER_MAX}
                step={1}
                value={displayOrder}
                onChange={(event) => setDisplayOrder(event.target.value)}
                disabled={isSaving}
                aria-invalid={displayOrderInvalid}
                aria-describedby="attribute-definition-display-order-description"
                className="min-h-11"
              />
              <p
                id="attribute-definition-display-order-description"
                className={displayOrderInvalid ? "text-xs text-destructive" : "text-xs text-muted-foreground"}
              >
                {displayOrderInvalid
                  ? ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.DISPLAY_ORDER_INVALID
                  : ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.DISPLAY_ORDER_DESCRIPTION}
              </p>
            </div>
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="attribute-definition-visibility">{ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.VISIBILITY}</Label>
              <select
                id="attribute-definition-visibility"
                className="flex min-h-11 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                value={visibility}
                onChange={(event) => setVisibility(event.target.value as AttributeVisibility)}
                disabled={isSaving}
              >
                <option value="public">{ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.VISIBILITY_PUBLIC}</option>
                <option value="private">{ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.VISIBILITY_PRIVATE}</option>
                <option value="admin" disabled={requiredOnOnboarding}>
                  {ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.VISIBILITY_ADMIN}
                </option>
              </select>
            </div>
            <div className="space-y-3 rounded-xl border border-border p-3">
              <label className="flex min-h-10 items-center gap-3 text-sm">
                <input
                  type="checkbox"
                  checked={userEditable}
                  onChange={(event) => setUserEditable(event.target.checked)}
                  disabled={isSaving || requiredOnOnboarding}
                />
                {ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.USER_EDITABLE}
              </label>
              <label className="flex min-h-10 items-center gap-3 text-sm">
                  <input
                    type="checkbox"
                    checked={requiredOnOnboarding}
                    onChange={(event) => {
                      const required = event.target.checked;
                      setRequiredOnOnboarding(required);
                      if (required) {
                        setUserEditable(true);
                        setVisibility((current) =>
                          current === "admin" ? "private" : current,
                        );
                      }
                    }}
                    disabled={isSaving || editing?.kind === "system"}
                  />
                  {ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.REQUIRED_ONBOARDING}
                </label>
              {editing?.kind === "system" ? (
                <p className="text-xs leading-relaxed text-muted-foreground">
                  {ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.REQUIRED_ONBOARDING_CODE_OWNED}
                </p>
              ) : null}
            </div>
          </div>
          <div className="space-y-2">
            <Label htmlFor="attribute-definition-description">{ADMIN_TEXT.FIELDS.DESCRIPTION}</Label>
            <Input
              id="attribute-definition-description"
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              placeholder={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.DESCRIPTION_PLACEHOLDER}
              maxLength={PROFILE_ATTRIBUTE_LIMITS.DESCRIPTION_MAX_LENGTH}
              disabled={isSaving}
              aria-invalid={descriptionInvalid}
              aria-describedby={descriptionInvalid ? "attribute-definition-description-error" : undefined}
            />
            {descriptionInvalid ? <p id="attribute-definition-description-error" className="text-sm text-destructive">{ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.DESCRIPTION_INVALID}</p> : null}
          </div>
          <div className="space-y-2">
            <Label htmlFor="attribute-definition-validation">{ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.VALIDATION}</Label>
            <Textarea
              id="attribute-definition-validation"
              className="min-h-28 font-mono text-xs"
              value={validation}
              onChange={(event) => setValidation(event.target.value)}
              disabled={isSaving}
              aria-invalid={parsedValidation === null}
            />
            <p className="text-xs text-muted-foreground">{ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.VALIDATION_HINT}</p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="attribute-definition-reason">{ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.REASON}</Label>
            <Textarea
              id="attribute-definition-reason"
              value={reason}
              onChange={(event) => setReason(event.target.value)}
              placeholder={ADMIN_TEXT.ATTRIBUTE_DEFINITIONS.REASON_PLACEHOLDER}
              maxLength={500}
              disabled={isSaving}
            />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose} disabled={isSaving} className="min-h-11">{TEXT.COMMON.CANCEL}</Button>
            <Button type="submit" disabled={(!editing && keyInvalid) || descriptionInvalid || revisionInvalid || isSaving} className="min-h-11">
              {isSaving ? <Loader2 className="mr-2 h-4 w-4 animate-spin motion-reduce:animate-none" aria-hidden="true" /> : null}
              {editing ? TEXT.COMMON.SAVE : TEXT.COMMON.CREATE}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

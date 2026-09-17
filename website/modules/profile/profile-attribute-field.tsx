"use client"

import { LockKeyhole, RotateCcw } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  PROFILE_ATTRIBUTE_LIMITS,
  isProfileAttributeNumber,
  profileAttributeOptionLabel,
} from "@/constants/profile-attributes"
import { TEXT } from "@/constants/text"
import type { ProfileAttribute } from "@/types/user"
import {
  profileAttributeEnumValues,
  type ProfileAttributeDraftValue,
  type ProfileAttributeFieldError,
} from "./profile-attribute-model"

interface ProfileAttributeFieldProps {
  attribute: ProfileAttribute
  value: ProfileAttributeDraftValue
  error?: ProfileAttributeFieldError
  disabled?: boolean
  allowUnset?: boolean
  onChange: (value: ProfileAttributeDraftValue) => void
}

export function ProfileAttributeField({
  attribute,
  value,
  error,
  disabled = false,
  allowUnset = false,
  onChange,
}: ProfileAttributeFieldProps) {
  const inputID = `profile-attribute-${attribute.definition_id}`
  const errorID = `${inputID}-error`
  const helpID = `${inputID}-help`
  const locked = disabled || !attribute.user_editable
  const enumValues = profileAttributeEnumValues(attribute)
  const describedBy = error ? errorID : attribute.description ? helpID : undefined

  return (
    <div className="rounded-2xl border border-border/60 bg-card/55 p-4 shadow-sm transition-colors focus-within:border-primary/45 focus-within:bg-card">
      <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <Label htmlFor={inputID} className="font-semibold">
              {attribute.label}
            </Label>
            {attribute.required_on_onboarding ? (
              <Badge variant="secondary" className="text-[10px] uppercase tracking-wide">
                {TEXT.PROFILE.ATTRIBUTES.REQUIRED}
              </Badge>
            ) : null}
            {!attribute.user_editable ? (
              <Badge variant="outline" className="gap-1 text-[10px] uppercase tracking-wide">
                <LockKeyhole className="size-3" aria-hidden="true" />
                {TEXT.PROFILE.ATTRIBUTES.READ_ONLY}
              </Badge>
            ) : null}
          </div>
          {attribute.description ? (
            <p id={helpID} className="text-xs leading-5 text-muted-foreground">
              {attribute.description}
            </p>
          ) : null}
        </div>
        {allowUnset && attribute.user_editable && value !== undefined ? (
          <Button
            type="button"
            variant="ghost"
            size="xs"
            disabled={disabled}
            onClick={() => onChange(undefined)}
            className="shrink-0 text-muted-foreground hover:text-foreground"
          >
            <RotateCcw className="size-3.5" aria-hidden="true" />
            {TEXT.PROFILE.ATTRIBUTES.UNSET}
          </Button>
        ) : null}
      </div>

      {attribute.data_type === "boolean" ? (
        <Select
          value={booleanSelectValue(value)}
          disabled={locked}
          onValueChange={(selected) => onChange(selected === "true")}
        >
          <SelectTrigger
            id={inputID}
            className="min-h-11 w-full"
            aria-invalid={Boolean(error)}
            aria-describedby={describedBy}
          >
            <SelectValue placeholder={TEXT.PROFILE.ATTRIBUTES.SELECT_PLACEHOLDER} />
          </SelectTrigger>
          <SelectContent>
            {booleanOptions(attribute).map((option) => (
              <SelectItem key={String(option)} value={String(option)}>
                {option
                  ? TEXT.PROFILE.ATTRIBUTES.BOOLEAN_TRUE
                  : TEXT.PROFILE.ATTRIBUTES.BOOLEAN_FALSE}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ) : enumValues.length > 0 ? (
        <Select
          value={selectedEnumIndex(enumValues, value)}
          disabled={locked}
          onValueChange={(index) => onChange(draftEnumValue(enumValues[Number(index)]))}
        >
          <SelectTrigger
            id={inputID}
            className="min-h-11 w-full"
            aria-invalid={Boolean(error)}
            aria-describedby={describedBy}
          >
            <SelectValue placeholder={TEXT.PROFILE.ATTRIBUTES.SELECT_PLACEHOLDER} />
          </SelectTrigger>
          <SelectContent>
            {enumValues.map((option, index) => (
              <SelectItem key={`${index}-${String(option)}`} value={String(index)}>
                {profileAttributeOptionLabel(attribute.key, option)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ) : (
        <Input
          id={inputID}
          type={inputType(attribute)}
          inputMode={attribute.data_type === "number" ? "decimal" : undefined}
          value={typeof value === "string" ? value : ""}
          disabled={locked}
          min={numberValidation(attribute.validation.minimum)}
          max={numberValidation(attribute.validation.maximum)}
          maxLength={
            attribute.data_type === "string"
              ? PROFILE_ATTRIBUTE_LIMITS.TEXT_INPUT_CODE_UNITS
              : undefined
          }
          step={attribute.data_type === "number" ? "any" : undefined}
          placeholder={TEXT.PROFILE.ATTRIBUTES.VALUE_PLACEHOLDER}
          aria-invalid={Boolean(error)}
          aria-describedby={describedBy}
          onChange={(event) => onChange(event.target.value)}
        />
      )}

      {error ? (
        <p id={errorID} className="mt-2 text-xs font-medium text-destructive" role="alert">
          {TEXT.PROFILE.ATTRIBUTES.FIELD_ERROR(error)}
        </p>
      ) : null}
    </div>
  )
}

function inputType(attribute: ProfileAttribute): "text" | "number" | "date" {
  if (attribute.data_type === "number") return "number"
  if (attribute.data_type === "date") return "date"
  return "text"
}

function selectedEnumIndex(
  values: Array<string | number | boolean>,
  draft: ProfileAttributeDraftValue,
): string | undefined {
  if (draft === undefined) return undefined
  const index = values.findIndex((value) => String(value) === String(draft))
  return index < 0 ? undefined : String(index)
}

function draftEnumValue(value: string | number | boolean | undefined): ProfileAttributeDraftValue {
  if (value === undefined || typeof value === "boolean") return value
  return String(value)
}

function booleanSelectValue(value: ProfileAttributeDraftValue): string | undefined {
  return typeof value === "boolean" ? String(value) : undefined
}

function booleanOptions(attribute: ProfileAttribute): boolean[] {
  const constrained = profileAttributeEnumValues(attribute).filter(
    (value): value is boolean => typeof value === "boolean",
  )
  return constrained.length > 0 ? constrained : [true, false]
}

function numberValidation(value: unknown): number | undefined {
  return isProfileAttributeNumber(value) ? value : undefined
}

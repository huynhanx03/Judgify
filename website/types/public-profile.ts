import type { EntityID, ISODateTime } from "@/types/api"

export const PUBLIC_PROFILE_ATTRIBUTE_DATA_TYPES = [
  "string",
  "number",
  "boolean",
  "date",
] as const

export type PublicProfileAttributeDataType =
  (typeof PUBLIC_PROFILE_ATTRIBUTE_DATA_TYPES)[number]

export type PublicProfileAttributeValue = string | number | boolean

export interface PublicProfileAttribute {
  key: string
  data_type: PublicProfileAttributeDataType
  label: string
  description: string
  value: PublicProfileAttributeValue
}

export interface PublicProfile {
  username: string
  joined_at: ISODateTime
  attributes: PublicProfileAttribute[]
}

export type ProfileVisibility = "private" | "public"

export interface ProfilePrivacy {
  visibility: ProfileVisibility
  version: number
  updated_at: ISODateTime
}

export interface UpdateProfilePrivacyInput {
  visibility: ProfileVisibility
  expected_version: number
}

export interface ProfilePrivacyMutationReceipt {
  command_id: EntityID
  event_id: EntityID
  state: ProfilePrivacy
  committed_at: ISODateTime
}

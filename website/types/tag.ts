/**
 * Tag-related types mirroring backend tag DTOs.
 */

import type { EntityID } from "@/types/api";

/** Element info nested inside a tag response. */
export interface TagElement {
  id: EntityID;
  name: string;
  code: string;
}

/** Tag entity returned by POST /tags/find and GET /tags/:id. */
export interface Tag {
  id: EntityID;
  name: string;
  elements: TagElement[];
  version: number;
}

/** POST /tags request body for creating a tag. */
export interface CreateTagRequest {
  name: string;
  element_ids?: EntityID[];
}

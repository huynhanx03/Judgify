/**
 * Tag-related types mirroring backend tag DTOs.
 */

/** Element info nested inside a tag response. */
export interface TagElement {
  id: number;
  name: string;
  code: string;
}

/** Tag entity returned by POST /tags/find and GET /tags/:id. */
export interface Tag {
  id: number;
  name: string;
  elements: TagElement[];
}

/** POST /tags request body for creating a tag. */
export interface CreateTagRequest {
  name: string;
  element_ids?: number[];
}

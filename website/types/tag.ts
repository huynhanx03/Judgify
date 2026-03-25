/**
 * Tag-related types mirroring backend tag DTOs.
 */

/** Tag entity returned by POST /tags/find and GET /tags/:id. */
export interface Tag {
  id: number;
  name: string;
}

/** POST /tags request body for creating a tag. */
export interface CreateTagRequest {
  name: string;
}

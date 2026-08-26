import type { EntityID } from "@/types/api";

/** Material category endpoints (Material module). */
export const MATERIAL_CATEGORY_API = {
  FIND_ALL: "/material-categories",
  FIND: "/material-categories/find",
  GET: (id: EntityID) => `/material-categories/${encodeURIComponent(id)}`,
  CREATE: "/material-categories",
  UPDATE: (id: EntityID) => `/material-categories/${encodeURIComponent(id)}`,
  DELETE: (id: EntityID) => `/material-categories/${encodeURIComponent(id)}`,
} as const;

/** Material article endpoints (Material module). */
export const MATERIAL_API = {
  SEARCH: "/materials/search",
  GET_BY_SLUG: (slug: string) => `/materials/slug/${encodeURIComponent(slug)}`,
  RELATED: (slug: string) => `/materials/slug/${encodeURIComponent(slug)}/related`,
  RECORD_VIEW: (id: EntityID) => `/materials/${encodeURIComponent(id)}/views`,
  ADMIN_SEARCH: "/admin/materials/search",
  GET_ADMIN: (id: EntityID) =>
    `/admin/materials/${encodeURIComponent(id)}`,
  REVISIONS: (id: EntityID) =>
    `/admin/materials/${encodeURIComponent(id)}/revisions`,
  REVISION: (id: EntityID, revisionId: EntityID) =>
    `/admin/materials/${encodeURIComponent(id)}/revisions/${encodeURIComponent(revisionId)}`,
  CREATE: "/admin/materials/drafts",
  UPDATE: (id: EntityID) => `/admin/materials/${encodeURIComponent(id)}/draft`,
	SUBMIT_REVIEW: (id: EntityID) => `/admin/materials/${encodeURIComponent(id)}/submit-review`,
	SUBMIT_REVIEW_PREVIEW: (id: EntityID) => `/admin/materials/${encodeURIComponent(id)}/submit-review/preview`,
	RETURN_TO_DRAFT: (id: EntityID) => `/admin/materials/${encodeURIComponent(id)}/return-to-draft`,
	RETURN_TO_DRAFT_PREVIEW: (id: EntityID) => `/admin/materials/${encodeURIComponent(id)}/return-to-draft/preview`,
  PUBLISH: (id: EntityID) => `/admin/materials/${encodeURIComponent(id)}/publish`,
  PUBLISH_PREVIEW: (id: EntityID) =>
    `/admin/materials/${encodeURIComponent(id)}/publish/preview`,
  ARCHIVE: (id: EntityID) => `/admin/materials/${encodeURIComponent(id)}/archive`,
  ARCHIVE_PREVIEW: (id: EntityID) =>
    `/admin/materials/${encodeURIComponent(id)}/archive/preview`,
  ROLLBACK: (id: EntityID) => `/admin/materials/${encodeURIComponent(id)}/rollback`,
  ROLLBACK_PREVIEW: (id: EntityID) =>
    `/admin/materials/${encodeURIComponent(id)}/rollback/preview`,
  RENAME: (id: EntityID) => `/admin/materials/${encodeURIComponent(id)}/rename`,
} as const;

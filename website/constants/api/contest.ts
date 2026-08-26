import type { EntityID } from "@/types/api";

/** Contest endpoints (Contest module). */
export const CONTEST_API = {
  FIND: "/contests/find",
  GET: (id: EntityID) => `/contests/${encodeURIComponent(id)}`,
  PROBLEM: (id: EntityID, contestProblemID: EntityID) =>
    `/contests/${encodeURIComponent(id)}/problems/${encodeURIComponent(contestProblemID)}`,
  ADMIN_FIND: "/admin/contests/find",
  ADMIN_GET: (id: EntityID) => `/admin/contests/${encodeURIComponent(id)}`,
  ADMIN_RERATE: (id: EntityID) =>
    `/admin/contests/${encodeURIComponent(id)}/rating/rerate`,
  ADMIN_END: (id: EntityID) =>
    `/admin/contests/${encodeURIComponent(id)}/end`,
  ADMIN_CANCEL: (id: EntityID) =>
    `/admin/contests/${encodeURIComponent(id)}/cancel`,
  ADMIN_STANDINGS: (id: EntityID) =>
    `/admin/contests/${encodeURIComponent(id)}/standings`,
  CREATE: "/contests",
  UPDATE: (id: EntityID) => `/contests/${encodeURIComponent(id)}`,
  PUBLISH: (id: EntityID) => `/contests/${encodeURIComponent(id)}/publish`,
  DELETE: (id: EntityID) => `/contests/${encodeURIComponent(id)}`,
  REGISTER: (id: EntityID) => `/contests/${encodeURIComponent(id)}/register`,
  UNREGISTER: (id: EntityID) =>
    `/contests/${encodeURIComponent(id)}/unregister`,
  STANDINGS: (id: EntityID) =>
    `/contests/${encodeURIComponent(id)}/standings`,
  MY_STANDING: (id: EntityID) =>
    `/contests/${encodeURIComponent(id)}/standings/me`,
  RATING_CHANGES: (id: EntityID) =>
    `/contests/${encodeURIComponent(id)}/rating-changes`,
  ANNOUNCEMENTS: (id: EntityID) => `/contests/${encodeURIComponent(id)}/announcements`,
  PARTICIPANT_ANNOUNCEMENTS: (id: EntityID) => `/contests/${encodeURIComponent(id)}/announcements/participants`,
  ADMIN_ANNOUNCEMENTS: (id: EntityID) =>
    `/admin/contests/${encodeURIComponent(id)}/announcements`,
  ADMIN_ANNOUNCEMENT: (id: EntityID, announcementID: EntityID) =>
    `/admin/contests/${encodeURIComponent(id)}/announcements/${encodeURIComponent(announcementID)}`,
  ADMIN_ANNOUNCEMENT_REVISIONS: (
    id: EntityID,
    announcementID: EntityID,
  ) =>
    `/admin/contests/${encodeURIComponent(id)}/announcements/${encodeURIComponent(announcementID)}/revisions`,
  CLARIFICATIONS: (id: EntityID) => `/contests/${encodeURIComponent(id)}/clarifications`,
  MY_CLARIFICATIONS: (id: EntityID) => `/contests/${encodeURIComponent(id)}/clarifications/mine`,
  MY_CLARIFICATION: (id: EntityID, clarificationID: EntityID) => `/contests/${encodeURIComponent(id)}/clarifications/mine/${encodeURIComponent(clarificationID)}`,
  WITHDRAW_CLARIFICATION: (id: EntityID, clarificationID: EntityID) =>
    `/contests/${encodeURIComponent(id)}/clarifications/${encodeURIComponent(clarificationID)}/withdraw`,
  ANNOUNCEMENT: (id: EntityID, announcementID: EntityID) =>
    `/contests/${encodeURIComponent(id)}/announcements/${encodeURIComponent(announcementID)}`,
  ANNOUNCEMENT_PUBLISH: (id: EntityID, announcementID: EntityID) =>
    `/contests/${encodeURIComponent(id)}/announcements/${encodeURIComponent(announcementID)}/publish`,
  ANNOUNCEMENT_WITHDRAW: (id: EntityID, announcementID: EntityID) =>
    `/contests/${encodeURIComponent(id)}/announcements/${encodeURIComponent(announcementID)}/withdraw`,
  ADMIN_CLARIFICATIONS: (id: EntityID) => `/admin/contests/${encodeURIComponent(id)}/clarifications`,
  ADMIN_CLARIFICATION: (id: EntityID, clarificationID: EntityID) => `/admin/contests/${encodeURIComponent(id)}/clarifications/${encodeURIComponent(clarificationID)}`,
  ADMIN_CLARIFICATION_ANSWER: (id: EntityID, clarificationID: EntityID) =>
    `/admin/contests/${encodeURIComponent(id)}/clarifications/${encodeURIComponent(clarificationID)}/answer`,
  ADMIN_CLARIFICATION_CLOSE: (id: EntityID, clarificationID: EntityID) =>
    `/admin/contests/${encodeURIComponent(id)}/clarifications/${encodeURIComponent(clarificationID)}/close`,
  ADMIN_CLARIFICATION_REVISIONS: (id: EntityID, clarificationID: EntityID) => `/admin/contests/${encodeURIComponent(id)}/clarifications/${encodeURIComponent(clarificationID)}/revisions`,
} as const;

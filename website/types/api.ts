/**
 * Generic API response types that mirror the backend response wrapper.
 * Backend wraps all responses in { code, message, data } format.
 */

/** Standard API response wrapper from backend. */
export interface ApiResponse<T> {
  code: number;
  message: string;
  data: T;
}

/** Pagination metadata returned by backend for list endpoints. */
export interface PaginationMeta {
  current_page: number;
  page_size: number;
  total_pages: number;
  total_items: number;
  has_next: boolean;
  has_prev: boolean;
}

/** Paginated data container returned by backend list endpoints. */
export interface Paginated<T> {
  records: T[];
  pagination: PaginationMeta;
}

/** Search/filter parameter for query endpoints. */
export interface SearchFilter {
  key: string;
  value: unknown;
  /** "search" | "filter" | "exact" */
  type: string;
}

/** Sort parameter for query endpoints. */
export interface SortOption {
  key: string;
  /** 1 = ascending, -1 = descending */
  order: number;
}

/** Combined query options sent to backend POST /find endpoints. */
export interface QueryOptions {
  pagination?: {
    page: number;
    page_size: number;
  };
  filters?: SearchFilter[];
  sort?: SortOption[];
}

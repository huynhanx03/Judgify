/**
 * The product currently ships one locale. Keeping its language and formatting
 * tag behind this stable module lets a future locale switch replace the
 * catalog and formatter configuration without editing feature components.
 */
export const APP_LANGUAGE = "vi" as const;
export const APP_LOCALE = "vi-VN" as const;

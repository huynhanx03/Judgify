const configuredApiUrl = process.env.NEXT_PUBLIC_API_URL?.trim();

/**
 * Browser traffic is same-origin in every environment. Local development uses
 * the same reverse-proxy/route contract as production instead of silently
 * switching credentials to a second origin.
 *
 * Server Components may set an absolute internal origin through the existing
 * deployment variable; the transport still refuses that origin in a browser
 * when it differs from window.location.origin.
 */
export const BASE_API_URL = (configuredApiUrl || "/api").replace(/\/$/, "");

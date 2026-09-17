export const CSRF_COOKIE_NAME = "judgify_csrf";
export const CSRF_HEADER_NAME = "X-CSRF-Token";

const MINIMUM_CSRF_LENGTH = 16;
const MAXIMUM_CSRF_LENGTH = 4096;
const CONTROL_CHARACTER_PATTERN = /[\u0000-\u001f\u007f]/;

export interface CsrfTokenSource {
  read(): string | null;
}

function validateCsrfToken(value: string): string {
  if (
    value.length < MINIMUM_CSRF_LENGTH ||
    value.length > MAXIMUM_CSRF_LENGTH ||
    CONTROL_CHARACTER_PATTERN.test(value)
  ) {
    throw new TypeError("invalid CSRF token boundary");
  }
  return value;
}

export function csrfTokenFromCookie(
  cookieHeader: string,
  cookieName = CSRF_COOKIE_NAME,
): string | null {
  let match: string | null = null;
  for (const segment of cookieHeader.split(";")) {
    const separator = segment.indexOf("=");
    if (separator < 0) continue;
    const name = segment.slice(0, separator).trim();
    if (name !== cookieName) continue;
    if (match !== null) throw new TypeError("duplicate CSRF cookie");
    const encoded = segment.slice(separator + 1).trim();
    let decoded: string;
    try {
      decoded = decodeURIComponent(encoded);
    } catch {
      throw new TypeError("invalid CSRF cookie encoding");
    }
    match = validateCsrfToken(decoded);
  }
  return match;
}

export function createBrowserCsrfTokenSource(): CsrfTokenSource {
  return {
    read(): string | null {
      if (typeof document === "undefined") return null;
      return csrfTokenFromCookie(document.cookie);
    },
  };
}

export function createStaticCsrfTokenSource(
  token: string | null,
): CsrfTokenSource {
  const validated = token === null ? null : validateCsrfToken(token);
  return { read: () => validated };
}

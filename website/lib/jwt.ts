/**
 * Client-side JWT decode utilities.
 * Decodes JWT payload without verifying signature (signature verified by BE).
 * Used for instant auth state on page load — no API call needed.
 */

/** JWT payload matching backend Claims struct. */
export interface JwtPayload {
  sub_int: number;
  username: string;
  type: "access" | "refresh";
  exp: number;
  iat: number;
  sub: string;
}

/**
 * Decodes a JWT token payload without signature verification.
 * Returns null if token is malformed.
 */
export function decodeJwt(token: string): JwtPayload | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;
    const payload = JSON.parse(atob(parts[1]));
    return payload as JwtPayload;
  } catch {
    return null;
  }
}

/** Checks if a JWT token is expired (with 30s buffer for clock skew). */
export function isTokenExpired(token: string): boolean {
  const payload = decodeJwt(token);
  if (!payload?.exp) return true;
  return Date.now() >= (payload.exp - 30) * 1000;
}

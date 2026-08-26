/**
 * Centralized toast notification system.
 * Wraps sonner's toast API so all components use a single entry point.
 * To change toast behavior/style globally, update this file only.
 */

import { toast as sonnerToast } from "sonner";

export const notify = {
  success: (message: string) => sonnerToast.success(message),
  error: (message: string) => sonnerToast.error(message),
  warning: (message: string) => sonnerToast.warning(message),
  info: (message: string) => sonnerToast.info(message),
};

/**
 * Return catalog-owned presentation copy. The error remains available to the
 * caller for diagnostics, but backend messages never cross the UI text boundary.
 */
export function getErrorMessage(_error: unknown, fallback: string): string {
  return fallback;
}

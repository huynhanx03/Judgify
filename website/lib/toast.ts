/**
 * Centralized toast notification system.
 * Wraps sonner's toast API so all components use a single entry point.
 * To change toast behavior/style globally, update this file only.
 */

import { toast as sonnerToast } from "sonner";
import { ApiError } from "@/lib/api-client";

export const notify = {
  success: (message: string) => sonnerToast.success(message),
  error: (message: string) => sonnerToast.error(message),
  warning: (message: string) => sonnerToast.warning(message),
  info: (message: string) => sonnerToast.info(message),
};

/** Extract a user-friendly message from an error, falling back to the given default. */
export function getErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError && err.message) return err.message;
  return fallback;
}

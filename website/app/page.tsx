/**
 * Root page — redirects to /arena as the main entry point.
 */

import { redirect } from "next/navigation";
import { APP_ROUTES } from "@/constants/routes";

/** Redirects to the Arena page. */
export default function HomePage() {
  redirect(APP_ROUTES.ARENA);
}

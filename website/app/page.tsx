/**
 * Root page — redirects to /arena as the main entry point.
 */

import { redirect } from "next/navigation";

/** Redirects to the Arena page. */
export default function HomePage() {
  redirect("/arena");
}

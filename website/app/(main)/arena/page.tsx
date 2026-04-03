/**
 * Arena page — displays a list of problems with filtering.
 * Server component fetches real data from API.
 */

import { TEXT } from "@/constants/text";
import { ArenaClient } from "./arena-client";

export const metadata = {
  title: TEXT.NAV.ARENA,
};

export const dynamic = "force-dynamic";

export default function ArenaPage() {
  return <ArenaClient />;
}

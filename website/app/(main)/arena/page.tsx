/**
 * Arena page — displays a list of problems with filtering.
 * Server component fetches real data from API.
 */

import { TEXT } from "@/constants/text";
import { ArenaPage } from "@/modules/arena/arena-page";

export const metadata = {
  title: TEXT.NAV.ARENA,
};

export const dynamic = "force-dynamic";

export default function ArenaRoute() {
  return <ArenaPage />;
}

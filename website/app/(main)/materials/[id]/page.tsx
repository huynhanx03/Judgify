import type { Metadata } from "next";
import { TEXT } from "@/constants/text";
import MaterialDetailPage from "@/modules/materials/material-detail-page";

export const dynamic = "force-dynamic";
export const metadata: Metadata = {
  title: TEXT.NAV.MATERIALS,
};

export default function MaterialDetailRoute() {
  return <MaterialDetailPage />;
}

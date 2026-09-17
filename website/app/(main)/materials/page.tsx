import type { Metadata } from "next";
import { TEXT } from "@/constants/text";
import MaterialsPage from "@/modules/materials/materials-page";

export const metadata: Metadata = {
  title: TEXT.NAV.MATERIALS,
};

export default function MaterialsRoute() {
  return <MaterialsPage />;
}

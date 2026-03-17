import { MOCK_MATERIALS } from "@/mock/materials";
import type { Material } from "@/types/material";

/**
 * Service for managing Library Materials (Tàng Kinh Các).
 * Simulated async behavior for future API integration.
 */
export const materialService = {
  /**
   * Fetch all available materials.
   */
  async getMaterials(): Promise<Material[]> {
    // Simulate network delay
    await new Promise((resolve) => setTimeout(resolve, 600));
    return MOCK_MATERIALS;
  },

  /**
   * Fetch a single material by ID.
   */
  async getMaterialById(id: number): Promise<Material | undefined> {
    await new Promise((resolve) => setTimeout(resolve, 300));
    return MOCK_MATERIALS.find((m) => m.id === id);
  },
};

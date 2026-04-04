"use client";

import { motion, AnimatePresence } from "framer-motion";
import { X, Loader2 } from "lucide-react";
import { Separator } from "@/components/ui/separator";
import { TEXT } from "@/constants/text";
import { TraitCard } from "./trait-card";
import type { TraitResponse } from "@/types/cultivation";

interface TraitCodexModalProps {
  open: boolean;
  onClose: () => void;
  traits: TraitResponse[];
  loading?: boolean;
}

/** Fullscreen modal showing all available traits — root bones on top, talents in 3-col grid below. */
export function TraitCodexModal({ open, onClose, traits, loading }: TraitCodexModalProps) {
  const rootBones = traits.filter((t) => t.type === "root_bone");
  const talents = traits.filter((t) => t.type === "talent");

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4"
          onClick={onClose}
        >
          <motion.div
            initial={{ y: 20, opacity: 0 }}
            animate={{ y: 0, opacity: 1 }}
            exit={{ y: 20, opacity: 0 }}
            transition={{ type: "spring", stiffness: 300, damping: 30 }}
            className="relative w-[90vw] max-w-6xl max-h-[85vh] overflow-y-auto bg-zinc-950/95 border border-white/10 rounded-2xl custom-scrollbar"
            onClick={(e) => e.stopPropagation()}
          >
            {/* Sticky Header */}
            <div className="sticky top-0 z-10 bg-zinc-950/90 backdrop-blur-lg border-b border-white/10 px-6 py-4 rounded-t-2xl">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-xl font-bold text-primary tracking-wide">Thiên Mệnh Thư Quán</h3>
                  <p className="text-xs text-muted-foreground mt-0.5">Tổng hợp Căn Cốt và Thiên Phú trong thế giới tu luyện</p>
                </div>
                <button onClick={onClose} className="text-muted-foreground hover:text-foreground transition-colors p-2 rounded-lg hover:bg-white/5">
                  <X className="w-5 h-5" />
                </button>
              </div>
            </div>

            {/* Content */}
            <div className="px-6 py-6">
              {loading ? (
                <div className="h-[60vh] flex items-center justify-center">
                  <Loader2 className="w-8 h-8 animate-spin text-primary" />
                </div>
              ) : (
                <>
                  {/* Root Bones — 3 cols */}
                  <p className="text-xs font-bold uppercase tracking-wider text-amber-400 mb-3">
                    🦴 {TEXT.AUTH.TRAIT_ROOT_BONE} ({rootBones.length})
                  </p>
                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-2 mb-6">
                    {rootBones.map((trait) => (
                      <TraitCard key={trait.id} trait={trait} variant="compact" />
                    ))}
                  </div>

                  <Separator className="bg-white/5 my-5" />

                  {/* Talents — 3 cols */}
                  <p className="text-xs font-bold uppercase tracking-wider text-purple-400 mb-3">
                    ✨ {TEXT.AUTH.TRAIT_TALENT} ({talents.length})
                  </p>
                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-2">
                    {talents.map((trait) => (
                      <TraitCard key={trait.id} trait={trait} variant="compact" />
                    ))}
                  </div>
                </>
              )}
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}

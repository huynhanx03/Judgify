import RegisterFlow from "@/modules/auth/RegisterFlow";
import { Card } from "@/components/ui/card";
import { Metadata } from "next";

export const metadata: Metadata = {
  title: "Khai Căn Trúc Cốt | Judgify",
};

export default function RegisterPage() {
  return (
    <Card className="glass-card shadow-2xl border-white/10 dark:border-white/5 bg-background/60 dark:bg-zinc-950/60 backdrop-blur-xl overflow-hidden relative w-full">
      {/* Decorative corners */}
      <div className="absolute top-0 left-0 w-16 h-16 border-t-2 border-l-2 border-primary/30 rounded-tl-xl pointer-events-none" />
      <div className="absolute top-0 right-0 w-16 h-16 border-t-2 border-r-2 border-primary/30 rounded-tr-xl pointer-events-none" />
      <div className="absolute bottom-0 left-0 w-16 h-16 border-b-2 border-l-2 border-primary/30 rounded-bl-xl pointer-events-none" />
      <div className="absolute bottom-0 right-0 w-16 h-16 border-b-2 border-r-2 border-primary/30 rounded-br-xl pointer-events-none" />

      <div className="p-5 lg:p-8 relative z-10">
        <RegisterFlow />
      </div>
    </Card>
  );
}

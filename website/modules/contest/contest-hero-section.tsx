/**
 * Hero banner for the Contest page — grand arena with call-to-action buttons.
 */

import { TEXT } from "@/constants/text";
import { Flame } from "lucide-react";
import Image from "next/image";

export function ContestHeroSection() {
  return (
    <div className="relative h-[450px] w-full overflow-hidden rounded-[3.5rem] border border-amber-500/20 shadow-[0_0_50px_rgba(245,158,11,0.15)] group">
      <Image
        src="/images/contest-bg.png"
        alt="Grand Arena"
        fill
        className="object-cover object-center brightness-[0.5] transition-transform duration-[2000ms] group-hover:scale-110"
        priority
      />
      <div className="absolute inset-0 bg-gradient-to-t from-background via-transparent to-black/60" />

      {/* Animated ambient light */}
      <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-full h-full bg-amber-500/10 mix-blend-overlay blur-[120px] animate-pulse" />

      <div className="absolute inset-0 flex flex-col items-center justify-center text-center p-6 space-y-6">
        <div className="inline-flex items-center gap-2 px-5 py-2 rounded-full bg-amber-500/20 border border-amber-500/40 text-amber-500 text-sm font-black tracking-widest uppercase animate-in fade-in zoom-in duration-700">
          <Flame className="h-4 w-4 fill-current animate-pulse" />
          ĐẠI HỘI QUẦN HÙNG
        </div>

        <div className="space-y-2 max-w-4xl">
          <h1 className="text-6xl md:text-8xl font-playfair italic font-bold tracking-tight text-white drop-shadow-[0_0_30px_rgba(245,158,11,0.5)] animate-in slide-in-from-bottom-8 duration-700">
            {TEXT.NAV.CONTEST}
          </h1>
          <p className="text-amber-100/80 text-xl md:text-2xl max-w-3xl mx-auto leading-relaxed animate-in slide-in-from-bottom-10 duration-700 delay-100">
            Nơi vạn tông hội tụ, thiên tài tranh phong. Khai mở bí cảnh, đoạt lấy thiên địa linh bảo và ghi danh trên Vạn Cổ Tiên Bảng.
          </p>
        </div>

        <div className="flex gap-4 pt-6 animate-in slide-in-from-bottom-12 duration-700 delay-200">
          <button className="px-8 py-4 bg-amber-500 text-black font-black rounded-2xl shadow-[0_0_20px_rgba(245,158,11,0.4)] hover:shadow-[0_0_30px_rgba(245,158,11,0.6)] hover:scale-105 active:scale-95 transition-all">
            ĐĂNG KÝ NGAY
          </button>
          <button className="px-8 py-4 bg-white/10 backdrop-blur-xl border border-white/20 text-white font-black rounded-2xl hover:bg-white/20 transition-all">
            XEM LỊCH THI ĐẤU
          </button>
        </div>
      </div>
    </div>
  );
}

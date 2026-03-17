"use client";

import { useEffect, useState } from "react";
import { TEXT } from "@/constants/text";
import { Badge } from "@/components/ui/badge";
import { materialService } from "@/services/material.service";
import type { Material } from "@/types/material";
import * as LucideIcons from "lucide-react";
import { Loader2, Search, Filter } from "lucide-react";
import Image from "next/image";

export default function MaterialsPage() {
  const [materials, setMaterials] = useState<Material[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const fetchMaterials = async () => {
      try {
        const data = await materialService.getMaterials();
        setMaterials(data);
      } catch (error) {
        console.error("Failed to fetch materials:", error);
      } finally {
        setIsLoading(false);
      }
    };
    fetchMaterials();
  }, []);

  if (isLoading) {
    return (
      <div className="flex h-[60vh] items-center justify-center">
        <Loader2 className="h-10 w-10 text-primary animate-spin" />
      </div>
    );
  }

  return (
    <div className="space-y-12 pb-20">
      {/* Hero Section with Custom Background */}
      <div className="relative h-[400px] w-full overflow-hidden rounded-[3rem] border border-border/40 shadow-2xl">
        <Image 
          src="/images/materials-bg.png" 
          alt="Sacred Library" 
          fill 
          className="object-cover object-center brightness-[0.4] transition-transform duration-1000 hover:scale-105"
          priority
        />
        <div className="absolute inset-0 bg-gradient-to-t from-background via-transparent to-transparent" />
        <div className="absolute inset-0 flex flex-col items-center justify-center text-center p-6 space-y-4">
          <Badge className="bg-amber-500/20 text-amber-500 border-amber-500/30 backdrop-blur-md px-4 py-1 text-sm font-bold animate-in fade-in zoom-in duration-500">
            TRUNG TÂM TRI THỨC
          </Badge>
          <h1 className="text-5xl md:text-7xl font-playfair italic font-bold tracking-tight text-white drop-shadow-[0_0_20px_rgba(255,255,255,0.3)] animate-in slide-in-from-bottom-6 duration-700">
            {TEXT.NAV.MATERIALS}
          </h1>
          <p className="text-zinc-300 text-lg max-w-2xl leading-relaxed animate-in slide-in-from-bottom-8 duration-700 delay-100">
            Nơi tập hợp hàng vạn cuốn bí tịch cổ xưa, từ những chiêu thức nhập môn đến những đại công pháp nghịch thiên cải mệnh.
          </p>
          
          <div className="flex items-center gap-4 pt-4 animate-in slide-in-from-bottom-10 duration-700 delay-200">
            <div className="relative group">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-zinc-400 group-focus-within:text-amber-500 transition-colors" />
              <input 
                type="text" 
                placeholder="Tìm bí tịch..." 
                className="bg-black/40 border border-zinc-700/50 backdrop-blur-xl rounded-xl pl-10 pr-4 py-2 text-white w-64 focus:outline-none focus:ring-2 focus:ring-amber-500 transition-all shadow-inner"
              />
            </div>
            <button className="h-10 w-10 flex items-center justify-center rounded-xl bg-zinc-800/80 border border-zinc-700/50 backdrop-blur-xl text-zinc-300 hover:bg-zinc-700 transition-all">
              <Filter className="h-4 w-4" />
            </button>
          </div>
        </div>
      </div>

      {/* Main Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8 max-w-7xl mx-auto px-4">
        {materials.map((material, index) => {
          const Icon = (LucideIcons as any)[material.iconName] || LucideIcons.Book;
          
          return (
            <div 
              key={material.id}
              className={`group relative overflow-hidden rounded-[2.5rem] border ${material.border} bg-card/30 p-8 transition-all duration-500 hover:-translate-y-2 hover:shadow-2xl hover:shadow-primary/5 animate-in fade-in slide-in-from-bottom-4`}
              style={{ animationDelay: `${index * 100}ms` }}
            >
              {/* Animated aura inside card */}
              <div className={`absolute -right-20 -top-20 h-40 w-40 rounded-full bg-gradient-to-br ${material.color} blur-3xl opacity-20 group-hover:opacity-40 transition-opacity duration-700`} />
              
              <div className="relative z-10 space-y-6">
                <div className="flex items-start justify-between">
                  <div className={`p-4 rounded-2xl bg-card border ${material.border} shadow-lg ${material.text} transition-transform duration-500 group-hover:scale-110 group-hover:rotate-6`}>
                    <Icon className="h-8 w-8" />
                  </div>
                  <div className="flex flex-col items-end gap-2">
                    <Badge variant="outline" className={`${material.text} border-current/20 bg-background/40 backdrop-blur-md rounded-full px-3`}>
                      {material.type}
                    </Badge>
                    <Badge className="bg-primary text-primary-foreground font-black text-xs px-2 shadow-sm rounded-md tracking-tighter">
                      {material.rank}
                    </Badge>
                  </div>
                </div>

                <div className="space-y-3">
                  <h3 className="text-2xl font-black text-foreground group-hover:text-primary transition-colors tracking-tight leading-none">
                    {material.title}
                  </h3>
                  <p className="text-muted-foreground/80 leading-relaxed text-[15px] font-medium min-h-[4.5rem]">
                    {material.description}
                  </p>
                </div>

                <div className="pt-2">
                  <button className="w-full inline-flex items-center justify-center rounded-2xl bg-foreground px-6 py-3.5 text-sm font-bold text-background transition-all hover:scale-[1.02] active:scale-95 shadow-xl hover:shadow-primary/20">
                    NGHIÊN CỨU BÍ TỊCH
                  </button>
                </div>
              </div>

              {/* Faded Background Pattern */}
              <div className="absolute right-4 bottom-4 h-24 w-24 opacity-[0.03] transition-all duration-700 group-hover:opacity-[0.08] group-hover:scale-150 group-hover:-rotate-12">
                <Icon className="h-full w-full" />
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}



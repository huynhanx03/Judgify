"use client";

/**
 * Admin dashboard overview — stats cards fetched from real API.
 * Users stat shows 0 — no BE endpoint yet (TODO: wire when available).
 */

import { useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Users, FileCode2, Tags, Shield, Loader2 } from "lucide-react";
import { problemService } from "@/services/problem.service";
import { tagService } from "@/services/tag.service";
import { roleService } from "@/services/role.service";
import { userService } from "@/services/user.service";

interface DashboardStats {
  totalProblems: number;
  totalTags: number;
  totalRoles: number;
  totalUsers: number;
}

const STAT_CARDS = [
  { key: "totalUsers" as const, label: "Người Dùng", icon: Users, color: "text-blue-500" },
  { key: "totalProblems" as const, label: "Bài Tập", icon: FileCode2, color: "text-emerald-500" },
  { key: "totalTags" as const, label: "Tags", icon: Tags, color: "text-amber-500" },
  { key: "totalRoles" as const, label: "Vai Trò", icon: Shield, color: "text-violet-500" },
];

export default function AdminDashboardPage() {
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const countQuery = { pagination: { page: 1, page_size: 1 } };
    Promise.all([
      problemService.find(countQuery),
      tagService.find(countQuery),
      roleService.getAll(),
      userService.find(countQuery),
    ])
      .then(([problems, tags, roles, users]) => {
        setStats({
          totalProblems: problems.pagination.total_items,
          totalTags: tags.pagination.total_items,
          totalRoles: roles.length,
          totalUsers: users.pagination.total_items,
        });
      })
      .catch(console.error)
      .finally(() => setIsLoading(false));
  }, []);

  if (isLoading) {
    return (
      <div className="flex h-[50vh] items-center justify-center">
        <Loader2 className="h-8 w-8 text-primary animate-spin" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Dashboard</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Tổng quan hệ thống Judgify
        </p>
      </div>

      {/* Stats grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {STAT_CARDS.map(({ key, label, icon: Icon, color }) => (
          <Card key={key} className="border-border">
            <CardHeader className="flex flex-row items-center justify-between pb-2 space-y-0">
              <CardTitle className="text-sm font-medium text-muted-foreground">
                {label}
              </CardTitle>
              <Icon className={`h-4 w-4 ${color}`} />
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold tabular-nums">
                {stats?.[key] ?? 0}
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Recent activity placeholder */}
      <Card className="border-border">
        <CardHeader>
          <CardTitle className="text-base">Hoạt Động Gần Đây</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {[
              { action: "Tạo bài tập mới", user: "teacher01", time: "5 phút trước" },
              { action: "Đăng ký tài khoản", user: "student03", time: "15 phút trước" },
              { action: "Cập nhật vai trò", user: "admin", time: "1 giờ trước" },
              { action: "Publish bài tập", user: "teacher01", time: "2 giờ trước" },
            ].map((item, i) => (
              <div key={i} className="flex items-center justify-between py-2 border-b border-border last:border-0">
                <div>
                  <p className="text-sm font-medium">{item.action}</p>
                  <p className="text-xs text-muted-foreground">bởi {item.user}</p>
                </div>
                <span className="text-xs text-muted-foreground">{item.time}</span>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

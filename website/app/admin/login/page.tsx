"use client";

/**
 * Admin login page — separate from user login.
 * Clean minimal design for admin access.
 */

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { login } from "@/services/auth.service";
import { Loader2, Shield } from "lucide-react";

export default function AdminLoginPage() {
  const router = useRouter();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setIsLoading(true);
    setError("");

    try {
      await login({ username, password });
      router.push("/admin");
    } catch {
      setError("Tên đăng nhập hoặc mật khẩu không đúng");
    } finally {
      setIsLoading(false);
    }
  }

  return (
    <div className="relative flex min-h-screen items-center justify-center bg-background">
      {/* Subtle background grid */}
      <div className="absolute inset-0 bg-[linear-gradient(to_right,#80808008_1px,transparent_1px),linear-gradient(to_bottom,#80808008_1px,transparent_1px)] bg-[size:32px_32px]" />

      <div className="z-10 w-full max-w-sm px-4">
        <Card className="border-border shadow-xl">
          <CardHeader className="text-center space-y-3 pb-4">
            <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10 text-primary">
              <Shield className="h-6 w-6" />
            </div>
            <div className="space-y-1">
              <CardTitle className="text-xl font-bold tracking-tight">
                Admin Panel
              </CardTitle>
              <CardDescription className="text-muted-foreground text-sm">
                Đăng nhập để truy cập bảng điều khiển
              </CardDescription>
            </div>
          </CardHeader>

          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-4">
              {error && (
                <div className="rounded-lg bg-destructive/10 px-3 py-2.5 text-sm text-destructive border border-destructive/20">
                  {error}
                </div>
              )}

              <div className="space-y-2">
                <Label htmlFor="username" className="text-xs font-medium text-muted-foreground">
                  Tên đăng nhập
                </Label>
                <Input
                  id="username"
                  type="text"
                  placeholder="admin"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  required
                  className="h-10"
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="password" className="text-xs font-medium text-muted-foreground">
                  Mật khẩu
                </Label>
                <Input
                  id="password"
                  type="password"
                  placeholder="••••••••"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  className="h-10"
                />
              </div>

              <Button
                type="submit"
                className="w-full h-10 font-semibold cursor-pointer"
                disabled={isLoading}
              >
                {isLoading ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
                {isLoading ? "Đang xác thực..." : "Đăng Nhập"}
              </Button>
            </form>
          </CardContent>
        </Card>

        <p className="text-center text-xs text-muted-foreground/50 mt-4">
          Judgify Administration Panel
        </p>
      </div>
    </div>
  );
}

import { readFile } from "node:fs/promises";
import path from "node:path";
import { expect, test } from "vitest";

const root = process.cwd();

async function source(relativePath: string) {
  return readFile(path.join(root, relativePath), "utf8");
}

test("amber flame is the semantic primary in both supported themes", async () => {
  const css = await source("app/globals.css");

  expect(css).toMatch(/--primary:\s*var\(--primitive-amber-700\)/);
  expect(css).toMatch(
    /\.dark\s*\{[\s\S]*?--primary:\s*var\(--primitive-amber-400\)/,
  );
  expect(css).toMatch(/--button-background:\s*var\(--primary\)/);
  expect(css).toMatch(/--cultivation:\s*var\(--primary\)/);
  expect(css).toMatch(/--info:\s*var\(--primitive-blue-700\)/);
});

test("product navigation and auth continue to consume roles rather than copied colours", async () => {
  const [header, auth, adminLogin, brandMark] = await Promise.all([
    source("modules/layout/header.tsx"),
    source("modules/auth/LoginForm.tsx"),
    source("modules/admin/admin-login-page.tsx"),
    source("components/brand-mark.tsx"),
  ]);

  expect(header).toContain('isActive\n                      ? "text-primary"');
  expect(header).toContain("bg-primary");
  expect(auth).toContain("button-brand-elevation");
  expect(adminLogin).toContain("<BrandMark");
  expect(adminLogin).toContain('className="h-11"');
  expect(brandMark).toContain('src="/images/judgify-mark.png"');
});

test("the shared public and admin chrome preserve one visual rail and unboxed brand", async () => {
  const [appShell, header, adminShell, adminHeader, authLayout, brandMark] =
    await Promise.all([
      source("modules/layout/app-shell.tsx"),
      source("modules/layout/header.tsx"),
      source("modules/admin/admin-shell.tsx"),
      source("modules/admin/admin-header.tsx"),
      source("app/(auth)/layout.tsx"),
      source("components/brand-mark.tsx"),
    ]);

  expect(appShell).toContain("max-w-[1400px]");
  expect(header).toContain("max-w-[1400px]");
  expect(adminShell).toContain("max-w-[1400px]");
  expect(adminHeader).toContain("max-w-[1400px]");
  expect(header).toContain('<BrandMark className="size-8" priority />');
  expect(header).not.toMatch(/logo\.png/);
  expect(header).toContain("<ThemeToggle />");
  expect(authLayout).toContain("<ThemeToggle />");
  expect(header).toMatch(/LogIn|UserPlus/);
  expect(brandMark).not.toMatch(/rounded|overflow-hidden|object-cover/);
});

test("public product routes use the one shared visual page lead", async () => {
  const [arena, materials, contest, ranking, about, lead] = await Promise.all([
    source("modules/arena/arena-page.tsx"),
    source("modules/materials/materials-page.tsx"),
    source("modules/contest/contest-page.tsx"),
    source("modules/ranking/ranking-page.tsx"),
    source("modules/about/about-page.tsx"),
    source("components/public-page-lead.tsx"),
  ]);

  for (const page of [arena, materials, contest, ranking, about]) {
    expect(page).toContain("PublicPageLead");
  }
  expect(arena).toContain("eyebrow={TEXT.ARENA.EYEBROW}");
  expect(materials).toContain("eyebrow={TEXT.MATERIALS.EYEBROW}");
  for (const page of [arena, materials, contest, ranking]) {
    expect(page).not.toMatch(/font-playfair|Sparkles|PublicHero|MediaHero/);
  }

  expect(lead).toContain('arena: "/images/arena-page-lead-v1.png"');
  expect(lead).toContain('materials: "/images/materials-page-lead-v1.png"');
  expect(lead).toContain('contest: "/images/contest-page-lead-v1.png"');
  expect(lead).toContain('ranking: "/images/ranking-page-lead-v1.png"');
  expect(lead).toContain('about: "/images/about-page-lead-v1.png"');
  expect(lead).toContain("object-right");
  expect(lead).toContain("PageHeader");
});

test("public content routes leave the shell to own the outer page rail", async () => {
  const [
    arena,
    materials,
    materialDetail,
    contest,
    ranking,
    about,
    notifications,
  ] = await Promise.all([
    source("modules/arena/arena-page.tsx"),
    source("modules/materials/materials-page.tsx"),
    source("modules/materials/material-detail-page.tsx"),
    source("modules/contest/contest-page.tsx"),
    source("modules/ranking/ranking-page.tsx"),
    source("modules/about/about-page.tsx"),
    source("modules/notifications/notification-center.tsx"),
  ]);

  for (const page of [
    arena,
    materials,
    materialDetail,
    contest,
    ranking,
    about,
  ]) {
    expect(page).not.toMatch(/mx-auto max-w-(?:7xl|6xl)/);
  }
  expect(arena).not.toMatch(/icon=\{Swords\}/);
  expect(about).not.toMatch(/about_bg|rounded-2xl border.*shadow/);
  expect(notifications).not.toMatch(/icon=\{Bell\}/);
});

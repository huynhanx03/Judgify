import assert from "node:assert/strict";
import { render, screen } from "@testing-library/react";
import { Plus } from "lucide-react";
import { test } from "vitest";

import { PageHeader } from "@/components/page-header";

test("page header presents one page title, context and the local primary action", () => {
  render(
    <PageHeader
      icon={Plus}
      eyebrow="Operations"
      title="Manage problems"
      description="Review and publish the problem catalog."
      actions={<button type="button">Create problem</button>}
    />,
  );

  assert.equal(screen.getAllByRole("heading", { level: 1 }).length, 1);
  assert.ok(screen.getByRole("heading", { name: "Manage problems" }));
  assert.ok(screen.getByText("Operations"));
  assert.ok(screen.getByText("Review and publish the problem catalog."));
  assert.ok(screen.getByRole("button", { name: "Create problem" }));
  assert.ok(screen.getByRole("heading", { name: "Manage problems" }).className.includes("text-balance"));
});

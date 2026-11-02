import assert from "node:assert/strict";
import { render, screen } from "@testing-library/react";
import { test } from "vitest";

import { TEXT } from "@/constants/text";
import { AdminDataTable } from "@/modules/admin/admin-data-table";

test("admin data table provides one labelled data viewport with semantic columns", () => {
  render(
    <AdminDataTable
      columns={[
        { key: "name", label: "Tên", render: ({ name }) => name },
        { key: "actions", label: "", render: () => <button type="button">Sửa</button> },
      ]}
      data={[{ id: 1, name: "Kiếm phổ" }]}
      keyExtractor={(item) => item.id}
    />,
  );

  const viewport = screen.getByRole("region", { name: TEXT.COMMON.DATA_TABLE });
  assert.equal(screen.getAllByRole("region", { name: TEXT.COMMON.DATA_TABLE }).length, 1);
  assert.ok(viewport.className.includes("overflow-y-auto"));
  assert.ok(viewport.className.includes("max-h-"));
  assert.ok(screen.getByRole("columnheader", { name: "Tên" }));
  assert.ok(viewport.querySelector("thead")?.className.includes("sticky"));
  assert.ok(screen.getByRole("columnheader", { name: /thao tác/i }));
  assert.ok(screen.getByRole("button", { name: "Sửa" }));
});

test("admin data table renders its empty message as a table row", () => {
  render(
    <AdminDataTable
      columns={[{ key: "name", label: "Tên", render: ({ name }) => name }]}
      data={[] as { id: number; name: string }[]}
      keyExtractor={(item) => item.id}
      emptyMessage="Chưa có bản ghi"
    />,
  );

  assert.ok(screen.getByRole("cell", { name: "Chưa có bản ghi" }));
});

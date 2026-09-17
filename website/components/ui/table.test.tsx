import assert from "node:assert/strict";
import { render, screen } from "@testing-library/react";
import { test } from "vitest";

import { Table, TableBody, TableCell, TableRow } from "@/components/ui/table";

test("a labelled table exposes one keyboard-focusable horizontal viewport", () => {
  render(
    <Table containerLabel="Danh sách bản ghi quản trị">
      <TableBody>
        <TableRow>
          <TableCell>Record</TableCell>
        </TableRow>
      </TableBody>
    </Table>,
  );

  const viewport = screen.getByRole("region", { name: "Danh sách bản ghi quản trị" });
  assert.equal(viewport.tabIndex, 0);
  assert.ok(viewport.className.includes("overflow-x-auto"));
  assert.equal(screen.getAllByRole("table").length, 1);
});

test("a table's existing accessible name also labels its scroll viewport", () => {
  render(
    <Table aria-label="Nhật ký kiểm toán">
      <TableBody>
        <TableRow>
          <TableCell>Record</TableCell>
        </TableRow>
      </TableBody>
    </Table>,
  );

  assert.equal(screen.getAllByRole("region", { name: "Nhật ký kiểm toán" }).length, 1);
  assert.equal(screen.getByRole("region", { name: "Nhật ký kiểm toán" }).tabIndex, 0);
});

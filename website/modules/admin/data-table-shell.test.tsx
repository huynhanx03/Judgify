import assert from "node:assert/strict";
import { fireEvent, render, screen } from "@testing-library/react";
import { test, vi } from "vitest";

import { DataTableShell } from "@/modules/admin/data-table-shell";

test("replaces a failed admin collection table with the shared unframed recovery state", () => {
  const retry = vi.fn();
  render(
    <DataTableShell
      title="Danh sách bài tập"
      createLabel="Tạo bài tập"
      canCreate={false}
      onCreateClick={() => undefined}
      searchValue=""
      onSearch={() => undefined}
      loadError="Không thể tải danh sách bài tập."
      onRetryLoad={retry}
      table={<table aria-label="Bảng bài tập"><tbody><tr><td>Hidden</td></tr></tbody></table>}
      confirmDialog={null}
    />,
  );

  const feedback = screen.getByRole("alert");
  assert.ok(!feedback.className.includes("border"));
  assert.equal(screen.queryByRole("table", { name: "Bảng bài tập" }), null);
  fireEvent.click(screen.getByRole("button", { name: "Thử lại" }));
  assert.equal(retry.mock.calls.length, 1);
});

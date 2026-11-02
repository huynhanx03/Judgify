import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { AuthSurface } from "@/modules/auth/auth-surface";

test("auth surface stays solid and provides one optional decorative frame", () => {
  const { container } = render(
    <AuthSurface framed>
      <p>credential form</p>
    </AuthSurface>,
  );

  expect(screen.getByText("credential form")).toBeVisible();
  expect(container.querySelector('[data-slot="card"]')).toHaveClass(
    "bg-surface",
    "border-border/80",
  );
  expect(container.querySelector('[aria-hidden="true"]')).toBeInTheDocument();
  expect(container.querySelector('[class*="backdrop-blur"]')).toBeNull();
});

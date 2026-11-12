import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { seedAuthStore } from "@/tests/seedAuthStore";
import UpdateTutorial from "../UpdateTutorial";
import { latestUpdate } from "../updateTutorial";

vi.mock("@/hooks/useNamespaceLegacy", () => ({ useNamespaceLegacy: () => false }));

const userId = "tutorial-test-user";
const key = `cloudmatehub:update-tutorial:${userId}:${latestUpdate.id}`;

function mount() {
  return render(<MemoryRouter><UpdateTutorial /></MemoryRouter>);
}

beforeEach(() => {
  localStorage.removeItem(key);
  seedAuthStore({ userId, role: "observer", isAdmin: false });
});
afterEach(() => localStorage.removeItem(key));

it("opens once for the current update and can be replayed", async () => {
  const user = userEvent.setup();
  const first = mount();
  const dialog = screen.getByRole("dialog", { name: "What’s new" });
  expect(within(dialog).getByText("Switch between workspaces")).toBeInTheDocument();
  await user.click(within(dialog).getByRole("button", { name: "Done" }));
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  first.unmount();

  mount();
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "What’s new" }));
  expect(screen.getByRole("dialog", { name: "What’s new" })).toBeInTheDocument();
});

it("shows administration steps to admins and allows moving back", async () => {
  seedAuthStore({ userId, role: "owner", isAdmin: true });
  const user = userEvent.setup();
  mount();
  const dialog = within(screen.getByRole("dialog", { name: "What’s new" }));
  await user.click(dialog.getByRole("button", { name: "Next" }));
  expect(dialog.getByText("Create a workspace and add members")).toBeInTheDocument();
  await user.click(dialog.getByRole("button", { name: "Back" }));
  expect(dialog.getByRole("link", { name: "Go to Workspaces" })).toHaveAttribute("href", "/workspaces");
});

import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { createTestWrapper } from "@/tests/wrapper";
import WireGuard from "../WireGuard";

const state = vi.hoisted(() => ({ isAdmin: false, userId: "user-one" }));
const request = vi.hoisted(() => vi.fn());
vi.mock("@/stores/authStore", () => ({ useAuthStore: (select: (s: typeof state) => unknown) => select(state) }));
vi.mock("../api", () => ({ vpnRequest: request }));
vi.mock("qrcode", () => ({ default: { toDataURL: async () => "data:image/png;base64,private-test-code" } }));

function mount() {
  return render(<MemoryRouter><WireGuard /></MemoryRouter>, { wrapper: createTestWrapper() });
}

beforeEach(() => {
  state.isAdmin = false;
  request.mockReset();
  request.mockImplementation(async (path: string) => {
    if (path === "/status") return { connected: true };
    if (path === "/session") return { url: "/api/vpn/dashboard/" };
    if (path.endsWith("/configuration")) return { file: "test-private-fixture", fileName: "test.conf" };
    return [];
  });
});

describe("WireGuard role views", () => {
  it("shows the user's empty state without admin controls", async () => {
    mount();
    expect(await screen.findByText("Your VPN starts here")).toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "Administration" })).not.toBeInTheDocument();
    expect(request).not.toHaveBeenCalledWith("/catalog");
  });

  it("opens the upstream workspace only after requesting an admin session", async () => {
    state.isAdmin = true;
    mount();
    await userEvent.click(screen.getByRole("tab", { name: "Administration" }));
    const iframe = await screen.findByTitle("WGDashboard administration");
    expect(iframe).toHaveAttribute("src", "/api/vpn/dashboard/");
    expect(request).toHaveBeenCalledWith("/session", { method: "POST" });
  });

  it("loads assignments through the dedicated Community user directory", async () => {
    state.isAdmin = true;
    mount();
    await userEvent.click(screen.getByRole("tab", { name: "User assignments" }));
    expect(await screen.findByText("Assign a configuration")).toBeInTheDocument();
    await waitFor(() => expect(request).toHaveBeenCalledWith("/users?search=&page=1"));
    expect(screen.getByRole("button", { name: "Assign to user" })).toBeDisabled();
  });

  it("does not fetch private peer content until the user requests it", async () => {
    request.mockImplementation(async (path: string) => {
      if (path === "/status") return { connected: true };
      if (path === "/assignments") return [{ id: "owned", label: "Laptop", configuration: "wg0" }];
      return { file: "test-private-fixture", fileName: "test.conf" };
    });
    mount();
    await screen.findByText("Laptop");
    expect(request).not.toHaveBeenCalledWith("/assignments/owned/configuration");
    await userEvent.click(screen.getByRole("button", { name: "QR code" }));
    expect(await screen.findByAltText("Private WireGuard configuration QR code")).toBeInTheDocument();
    expect(screen.queryByText("test-private-fixture")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Hide QR code" }));
    await waitFor(() => expect(screen.queryByAltText("Private WireGuard configuration QR code")).not.toBeInTheDocument());
  });
});

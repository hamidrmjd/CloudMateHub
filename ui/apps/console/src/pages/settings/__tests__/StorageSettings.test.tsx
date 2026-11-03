import { beforeEach, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { createTestWrapper } from "@/tests/wrapper";
import StorageSettings from "../StorageSettings";
const state = vi.hoisted(() => ({ isAdmin: true, userId: "fixture-admin", token: "fixture-token" }));
vi.mock("@/stores/authStore", () => ({ useAuthStore: Object.assign((select: (s: typeof state) => unknown) => select(state), { getState: () => state }) }));
const status = { config: { id: "fixture-local", kind: "local", path: "/recordings/files" }, recording_required: true, last_error: "", last_success: "", pending: 0, capture_buffer: "PostgreSQL" };
beforeEach(() => { state.isAdmin = true; vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify(status), { status: 200 }))); });
function mount() { return render(<MemoryRouter initialEntries={["/settings/storage"]}><Routes><Route path="/settings/storage" element={<StorageSettings />} /><Route path="/settings/general" element={<div>General settings</div>} /></Routes></MemoryRouter>, { wrapper: createTestWrapper() }); }
it("denies members without fetching storage credentials", () => { state.isAdmin = false; mount(); expect(screen.getByText("General settings")).toBeInTheDocument(); expect(fetch).not.toHaveBeenCalled(); });
it("shows required recording and the durable recovery copy", async () => { mount(); expect(await screen.findByDisplayValue("/recordings/files")).toBeInTheDocument(); expect(screen.getByText(/Session recording is required/)).toBeInTheDocument(); expect(screen.getByText(/captured output remain in PostgreSQL/)).toBeInTheDocument(); });
it("retains settings when the destination fails verification", async () => {
  const fetcher = vi.fn(async (_path: unknown, options?: RequestInit) => options?.method === "PUT" ? new Response(JSON.stringify({ error: "storage write failed; settings were not changed" }), { status: 400 }) : new Response(JSON.stringify(status), { status: 200 })); vi.stubGlobal("fetch", fetcher);
  mount(); await screen.findByDisplayValue("/recordings/files"); await userEvent.click(screen.getByRole("button", { name: "Verify and save" })); expect(await screen.findByText(/storage write failed/)).toBeInTheDocument(); expect(screen.getByDisplayValue("/recordings/files")).toBeInTheDocument();
});
it("explains NFS mounting and refuses S3 secrets over HTTP", async () => { mount(); await screen.findByDisplayValue("/recordings/files"); await userEvent.selectOptions(screen.getByLabelText("Destination"), "nfs"); expect(screen.getByDisplayValue("/mnt/recordings")).toBeInTheDocument(); expect(screen.getByText(/An operator must mount the NFS share/)).toBeInTheDocument(); await userEvent.selectOptions(screen.getByLabelText("Destination"), "s3"); expect(screen.getByLabelText("Secret key")).toBeDisabled(); expect(screen.getByText(/Enable trusted HTTPS before entering S3 credentials/)).toBeInTheDocument(); });
it("tests storage without saving", async () => { const fetcher = vi.fn(async () => new Response(JSON.stringify(status), { status: 200 })); vi.stubGlobal("fetch", fetcher); mount(); await screen.findByDisplayValue("/recordings/files"); await userEvent.click(screen.getByRole("button", { name: "Test storage" })); expect(await screen.findByText(/Storage verified: write, read and cleanup succeeded/)).toBeInTheDocument(); expect(fetcher.mock.calls.some((args: unknown[]) => args[0] === "/api/storage/test")).toBe(true); });

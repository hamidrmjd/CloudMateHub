import { beforeEach, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { createTestWrapper } from "@/tests/wrapper";
import HttpsSettings from "../HttpsSettings";
const state = vi.hoisted(() => ({ isAdmin: true, userId: "fixture-admin", token: "fixture-token" }));
vi.mock("@/stores/authStore", () => ({ useAuthStore: Object.assign((select: (s: typeof state) => unknown) => select(state), { getState: () => state }) }));
const status = { host: "hub.example.com", https_url: "https://hub.example.com:8443/", active: "", previous: "", pending: "", deadline: "", secure_upload: false, certificates: [], history: [] };
beforeEach(() => {
  state.isAdmin = true;
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify(status), { status: 200 })));
});
function mount() { return render(<MemoryRouter initialEntries={["/settings/https"]}><Routes><Route path="/settings/https" element={<HttpsSettings />} /><Route path="/settings/general" element={<div>General settings</div>} /></Routes></MemoryRouter>, { wrapper: createTestWrapper() }); }
it("refuses member access without fetching certificates", () => { state.isAdmin = false; mount(); expect(screen.getByText("General settings")).toBeInTheDocument(); expect(fetch).not.toHaveBeenCalled(); });
it("guides HTTP users to local recovery and prevents insecure uploads", async () => { mount(); await screen.findByText("hub.example.com"); expect(screen.getByRole("button", { name: "Validate certificate" })).toBeDisabled(); expect(screen.getByRole("link", { name: "http://localhost:9080" })).toHaveAttribute("href", "http://localhost:9080"); });
it("shows certificate expiry and restore history without exposing keys", async () => {
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ ...status, secure_upload: true, active: "active", previous: "prior", certificates: [{ id: "active", trust: "system", expires: new Date(Date.now() + 5 * 86400000).toISOString() }], history: [{ at: new Date().toISOString(), action: "confirmed" }] }), { status: 200 })));
  mount(); expect(await screen.findByText(/expires in 5 days/i)).toBeInTheDocument(); expect(screen.getByText("Restore history")).toBeInTheDocument(); expect(screen.getByRole("button", { name: "Restore previous / cancel activation" })).toBeEnabled();
});
it("keeps a validation error visible without activating anything", async () => {
  const fetcher = vi.fn(async (_path: unknown, options?: RequestInit) => options?.method === "POST" ? new Response(JSON.stringify({ error: "certificate and private key do not match" }), { status: 400 }) : new Response(JSON.stringify({ ...status, secure_upload: true }), { status: 200 }));
  vi.stubGlobal("fetch", fetcher); mount(); await screen.findByText("hub.example.com");
  const cert = new File(["fixture-cert"], "certificate.pem", { type: "application/x-pem-file" }); const key = new File(["fixture-key"], "private.key", { type: "application/x-pem-file" });
  Object.defineProperty(cert, "text", { value: async () => "fixture-cert" }); Object.defineProperty(key, "text", { value: async () => "fixture-key" });
  await userEvent.upload(screen.getByLabelText("Certificate and intermediate chain (PEM)"), cert); await userEvent.upload(screen.getByLabelText("Private key (PEM)"), key); await userEvent.click(screen.getByRole("button", { name: "Validate certificate" }));
  expect(await screen.findByText("certificate and private key do not match")).toBeInTheDocument(); expect(fetcher.mock.calls.some(([path]) => String(path).includes("/activate"))).toBe(false);
});

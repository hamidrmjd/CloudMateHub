import { beforeEach, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { http, HttpResponse } from "msw";
import { server, jsonWithTotal } from "@/tests/msw";
import { createTestWrapper } from "@/tests/wrapper";
import { mockDevice, mockNamespace } from "@/tests/factories";
import { seedAuthStore } from "@/tests/seedAuthStore";
import { useAuthStore } from "@/stores/authStore";
import { useTerminalStore, type TerminalSession } from "@/stores/terminalStore";
import BrowserConsole from "../BrowserConsole";
const mode = vi.hoisted(() => ({ legacy: true }));
vi.mock("@/hooks/useNamespaceLegacy", () => ({ useNamespaceLegacy: () => mode.legacy }));
vi.mock("@/hooks/useDebouncedValue", () => ({ useDebouncedValue: <T,>(value: T) => value }));
vi.mock("@/components/ConnectModal", () => ({ default: ({ deviceName, deviceUid, onClose }: {deviceName:string;deviceUid:string;onClose:()=>void}) => <div role="dialog" aria-label="Connect device"><p>{deviceName}:{deviceUid}</p><button onClick={onClose}>Cancel connection</button></div> }));
const online = mockDevice({uid:"fixture-online",name:"client-linux",online:true});
const offline = mockDevice({uid:"fixture-offline",name:"offline-linux",online:false});
function mount() { return render(<MemoryRouter><BrowserConsole /></MemoryRouter>, {wrapper:createTestWrapper()}); }
function session(overrides: Partial<TerminalSession> = {}): TerminalSession { return {id:"fixture-session",deviceUid:online.uid,deviceName:online.name,tenant:"tenant-456",username:"client",password:"",state:"minimized",connectionStatus:"connected",...overrides}; }
beforeEach(() => {
  mode.legacy=true;seedAuthStore({role:"operator",isAdmin:false});useTerminalStore.setState({sessions:[]});
  server.use(http.get("*/api/namespaces/:tenant",()=>HttpResponse.json(mockNamespace())),http.get("*/api/devices",()=>jsonWithTotal([online,offline])));
});
it("lets a non-admin operator open the browser connection dialog",async()=>{mount();await screen.findByText("client-linux");await userEvent.click(screen.getAllByRole("button",{name:"Open console"})[0]);expect(screen.getByRole("dialog",{name:"Connect device"})).toHaveTextContent("client-linux:fixture-online");expect(screen.getByText(/Terminal sessions are recorded/)).toBeInTheDocument();});
it("prevents opening offline devices",async()=>{mount();await screen.findByText("offline-linux");expect(screen.getAllByRole("button",{name:"Open console"})[1]).toBeDisabled();});
it("resumes a current namespace session instead of opening another login",async()=>{useTerminalStore.setState({sessions:[session(),session({id:"foreign-session",deviceName:"foreign-secret-device",tenant:"other-tenant"})]});mount();await screen.findByText("client-linux");expect(screen.queryByText(/foreign-secret-device/)).not.toBeInTheDocument();await userEvent.click(screen.getByRole("button",{name:"Resume console"}));expect(useTerminalStore.getState().sessions.find(s=>s.id==="fixture-session")?.state).toBe("shown");expect(screen.queryByRole("dialog")).not.toBeInTheDocument();});
it("keeps observer access read-only and does not fetch console targets",async()=>{seedAuthStore({role:"observer",isAdmin:false});const calls=vi.fn();server.use(http.get("*/api/devices",()=>{calls();return jsonWithTotal([online]);}));mount();expect(screen.getByText(/Ask a namespace administrator for Operator access/)).toBeInTheDocument();expect(screen.queryByRole("button",{name:"Open console"})).not.toBeInTheDocument();expect(calls).not.toHaveBeenCalled();});
it("clears a pending connection when the namespace changes",async()=>{mount();await screen.findByText("client-linux");await userEvent.click(screen.getAllByRole("button",{name:"Open console"})[0]);expect(screen.getByRole("dialog")).toBeInTheDocument();act(()=>useAuthStore.setState({tenant:"other-tenant"}));await waitFor(()=>expect(screen.queryByRole("dialog")).not.toBeInTheDocument());});
it("explains the HTTPS requirement for browser keys on HTTP",async()=>{mode.legacy=false;vi.stubGlobal("isSecureContext",false);mount();await screen.findByText("client-linux");expect(screen.getByText(/browser SSH keys, which require HTTPS or localhost/)).toBeInTheDocument();expect(screen.getAllByRole("button",{name:"Open console"})[0]).toBeDisabled();vi.unstubAllGlobals();});
it("queries only accepted devices and searches with the existing scoped API",async()=>{const seen:string[]=[];server.use(http.get("*/api/devices",({request})=>{seen.push(request.url);return jsonWithTotal([online],13);}));mount();await screen.findByText("client-linux");await userEvent.type(screen.getByRole("textbox",{name:"Find a device"}),"laptop");await waitFor(()=>expect(seen.some(url=>new URL(url).searchParams.has("filter"))).toBe(true));expect(seen.every(url=>new URL(url).searchParams.get("status")==="accepted")).toBe(true);await userEvent.click(screen.getByRole("button",{name:"Next"}));await waitFor(()=>expect(seen.some(url=>new URL(url).searchParams.get("page")==="2")).toBe(true));});
it("shows a useful empty state without creating any device",async()=>{server.use(http.get("*/api/devices",()=>jsonWithTotal([])));mount();expect(await screen.findByText("No connected devices yet")).toBeInTheDocument();expect(screen.getByRole("link",{name:"View devices"})).toHaveAttribute("href","/devices");});

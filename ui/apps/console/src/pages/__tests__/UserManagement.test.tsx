import { beforeEach, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { createTestWrapper } from "@/tests/wrapper";
import UserManagement from "../UserManagement";
const state=vi.hoisted(()=>({isAdmin:true,userId:"fixture-admin",token:"fixture-token"}));
vi.mock("@/stores/authStore",()=>({useAuthStore:Object.assign((selector:(s:typeof state)=>unknown)=>selector(state),{getState:()=>state})}));
beforeEach(()=>{state.isAdmin=true;vi.stubGlobal("fetch",vi.fn(async()=>new Response(JSON.stringify({users:["local","ad","ldap"].map((origin)=>({id:origin,username:`fixture.${origin}`,name:`Fixture ${origin}`,email:`${origin}@example.com`,origin,admin:origin==="local",status:"confirmed"})),total:3,per_page:25}),{status:200})));});
function mount(){render(<MemoryRouter initialEntries={["/users"]}><Routes><Route path="/users" element={<UserManagement/>}/><Route path="/dashboard" element={<div>Dashboard</div>}/></Routes></MemoryRouter>,{wrapper:createTestWrapper()});}
it("shows all three account sources and administrator access",async()=>{mount();expect(await screen.findByText("fixture.ad")).toBeInTheDocument();expect(screen.getByText("Local")).toBeInTheDocument();expect(screen.getByText("Active Directory")).toBeInTheDocument();expect(screen.getByText("OpenLDAP")).toBeInTheDocument();expect(screen.getByText("Administrator")).toBeInTheDocument();});
it("blocks members from the user directory",()=>{state.isAdmin=false;mount();expect(screen.getByText("Dashboard")).toBeInTheDocument();expect(fetch).not.toHaveBeenCalled();});
it("routes directory account creation to connection settings",async()=>{mount();await screen.findByText("fixture.ad");await userEvent.click(screen.getByRole("button",{name:"Add user"}));await userEvent.selectOptions(screen.getByLabelText("Account type"),"ad");expect(screen.getByText(/Directory passwords stay managed/)).toBeInTheDocument();expect(screen.queryByLabelText("password")).not.toBeInTheDocument();});

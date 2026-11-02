import { describe, it, expect, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { useAuthStore } from "@/stores/authStore";
import ProtectedRoute from "../ProtectedRoute";

beforeEach(() => {
  useAuthStore.setState({
    token: null,
    user: null,
    userId: null,
    email: null,
    tenant: null,
    role: null,
    name: null,
    loading: false,
  });
});

function renderWithRouter(token: string | null) {
  useAuthStore.setState({ token });

  return render(
    <MemoryRouter initialEntries={["/protected"]}>
      <Routes>
        <Route element={<ProtectedRoute />}>
          <Route path="/protected" element={<div>secret content</div>} />
        </Route>
        <Route path="/login" element={<div>login page</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("ProtectedRoute", () => {
  it("renders child route when token exists", () => {
    renderWithRouter("valid-token");
    expect(screen.getByText("secret content")).toBeInTheDocument();
  });

  it("redirects to /login when no token", () => {
    renderWithRouter(null);
    expect(screen.queryByText("secret content")).not.toBeInTheDocument();
    expect(screen.getByText("login page")).toBeInTheDocument();
  });
});

function LoginLocation() {
  const location = useLocation();
  return <div>{new URLSearchParams(location.search).get("redirect")}</div>;
}

it("preserves an SSH approval destination through sign-in", () => {
  const destination = "/ssh-identities/new/ABCDEFGH?source=terminal#review";
  render(
    <MemoryRouter initialEntries={[destination]}>
      <Routes>
        <Route element={<ProtectedRoute />}>
          <Route path="/ssh-identities/new/:code" element={<div>approval</div>} />
        </Route>
        <Route path="/login" element={<LoginLocation />} />
      </Routes>
    </MemoryRouter>,
  );
  expect(screen.getByText(destination)).toBeInTheDocument();
});

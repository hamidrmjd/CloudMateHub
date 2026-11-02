import { Outlet, Navigate, useLocation } from "react-router-dom";
import { useAuthStore } from "@/stores/authStore";
import ErrorBoundary from "./ErrorBoundary";
import { loginPathFor } from "@/utils/navigation";

/**
 * Gates a route on being signed in, sending anyone else to the sign-in screen with the current
 * path as the redirect, so they return where they were going.
 */
export default function ProtectedRoute() {
  const token = useAuthStore((s) => s.token);
  const { pathname, search, hash } = useLocation();

  if (!token) {
    return <Navigate to={loginPathFor(pathname, search, hash)} replace />;
  }

  return (
    <ErrorBoundary>
      <Outlet />
    </ErrorBoundary>
  );
}

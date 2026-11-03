import { useState, useEffect } from "react";
import {
  useNavigate,
  Navigate,
  useLocation,
  useSearchParams,
} from "react-router-dom";
import { ArrowRightEndOnRectangleIcon } from "@heroicons/react/24/outline";
import { Button, Callout, Spinner } from "@cloudmatehub/design-system/primitives";
import { useAuthStore } from "../stores/authStore";
import { isCloud, isEnterprise } from "../env";
import { getSafeRedirect } from "@/utils/navigation";
import PendingDeviceCallout from "@/components/auth/PendingDeviceCallout";
import AuthActions from "@/components/auth/AuthActions";
import SignInForm from "@/components/auth/SignInForm";
import ScreenIntro from "@/components/layout/ScreenIntro";
import { readLoginState } from "@/components/auth/loginState";
import { getInfo, getSamlAuthUrl } from "../client";

/**
 * The sign-in page. What it offers depends on the edition (SSO exists only on enterprise, sign-up
 * only on cloud) and on where the user was going, which it returns them to afterwards. Reached with
 * a {@link signOutLoginState} in the location state, it ends the current session itself and keeps
 * the notice beside it.
 */
export default function Login() {
  const isCloudEdition = isCloud();
  const isEnterpriseEdition = isEnterprise();
  const location = useLocation();
  const { notice, signOut } = readLoginState(location.state);

  const [searchParams] = useSearchParams();
  const queryToken = searchParams.get("token");
  const missingAssertions = searchParams.get("missing_assertions");
  const [tokenLoading, setTokenLoading] = useState(!!queryToken);
  const [authentication, setAuthentication] = useState<{
    local?: boolean;
    saml?: boolean;
  } | null>(null);
  const [directories, setDirectories] = useState<{kind: "ad" | "openldap"; label: string}[]>([]);
  const [provider, setProvider] = useState<"local" | "ad" | "openldap">("local");
  const [ssoLoading, setSsoLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (notice) {
      window.history.replaceState({}, document.title);
    }
  }, [notice]);

  useEffect(() => {
    if (signOut) {
      useAuthStore.getState().logout();
    }
  }, [signOut]);

  useEffect(() => {
    void fetch("/api/auth/directories", { cache: "no-store" }).then((r) => r.ok ? r.json() : []).then(setDirectories).catch(() => setDirectories([]));
    void getInfo()
      .then(({ data }) => setAuthentication(data?.authentication ?? null))
      .catch(() => setAuthentication(null));
  }, []);

  const token = useAuthStore((s) => s.token);
  const navigate = useNavigate();

  useEffect(() => {
    if (!queryToken) return;

    const { logout, loginWithToken } = useAuthStore.getState();
    logout();

    loginWithToken(queryToken)
      .then(() => navigate("/dashboard"))
      .catch(() => {
        setTokenLoading(false);
        setError("Failed to authenticate with the provided token.");
      });
  }, [queryToken, navigate]);

  const handleSsoLogin = async () => {
    setSsoLoading(true);
    try {
      const { data } = await getSamlAuthUrl({ throwOnError: true });
      window.location.replace(data.url);
    } catch {
      setError("Failed to retrieve SSO login URL. Please try again.");
      setSsoLoading(false);
    }
  };

  const showLocalForm = !isEnterpriseEdition || authentication?.local === true;
  const ssoOnly = isEnterpriseEdition && authentication?.local === false;

  if (token && !queryToken && !signOut) {
    return <Navigate to="/" replace />;
  }

  if (tokenLoading) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <Spinner size="xl" />
      </div>
    );
  }

  return (
    <>
      <ScreenIntro
        eyebrow="Sign in"
        title="Sign in to CloudMateHub"
        lead="Access your devices, sessions, and security rules from a single dashboard."
      />

      <div className="flex flex-col gap-3 mb-6 empty:hidden">
        <PendingDeviceCallout />
        {notice && <Callout variant="success">{notice}</Callout>}
        {missingAssertions && (
          <Callout variant="error">
            The SSO configuration is incomplete due to missing required
            mappings. Please contact your administrator.
          </Callout>
        )}
        {error && <Callout variant="error">{error}</Callout>}
      </div>

      {isEnterpriseEdition && authentication?.saml && (
        <div>
          <Button
            variant={ssoOnly ? "primary" : "secondary"}
            fullWidth
            loading={ssoLoading}
            disabled={ssoLoading}
            icon={<ArrowRightEndOnRectangleIcon className="w-4 h-4" />}
            data-testid="sso-btn"
            onClick={() => void handleSsoLogin()}
          >
            Login with SSO
          </Button>

          {!ssoOnly && (
            <div className="flex items-center gap-3 my-5">
              <div className="flex-1 h-px bg-border" />
              <span className="text-2xs font-mono text-text-muted uppercase tracking-label">
                or
              </span>
              <div className="flex-1 h-px bg-border" />
            </div>
          )}
        </div>
      )}

      {directories.length > 0 && <label className="block mb-5 text-sm">Account type<select aria-label="Account type" className="block mt-1 w-full rounded border border-border bg-background px-3 py-2" value={provider} onChange={(e) => setProvider(e.target.value as typeof provider)}><option value="local">Local</option>{directories.map((d) => <option key={d.kind} value={d.kind}>{d.label}</option>)}</select></label>}
      {provider !== "local" && window.location.protocol !== "https:" && <Callout variant="warning">Use trusted HTTPS to sign in with a directory account.</Callout>}
      {(showLocalForm || provider !== "local") && (
        <SignInForm
          key={provider}
          provider={provider === "local" ? undefined : provider}
          redirect={getSafeRedirect(new URLSearchParams(location.search))}
        />
      )}

      {isCloudEdition && (
        <div className="mt-4">
          <AuthActions
            links={[
              { label: "Forgot password?", to: "/forgot-password" },
              { label: "Don't have an account? Sign up", to: "/sign-up" },
            ]}
          />
        </div>
      )}
    </>
  );
}

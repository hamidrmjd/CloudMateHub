# Console and design

The user-facing application lives in `ui/apps/console`. It is a **React + TypeScript** SPA built by Vite. Production does not run a Vite development server: a `static-web-server` container serves the compiled assets.

```mermaid
flowchart TD
  Config["Runtime /config.json"] --> Entry["main.tsx"]
  Entry --> Router["React Router / App.tsx"]
  Router --> Layout["Public or signed-in layout"]
  Layout --> Page["Pages and components"]
  Page --> Hooks["Hooks + TanStack Query"]
  Hooks --> SDK["Generated OpenAPI client"]
  SDK --> API["Same-origin API via gateway"]
  Page <--> Store["Zustand state / browser persistence"]
  Page --> Terminal["xterm browser terminal"]
```

## Main layers

- **Pages and components:** user journeys such as login, setup, device inventory, sessions, access policies and account settings.
- **Hooks / TanStack Query:** requests, loading/error handling and server-state caching.
- **Zustand stores:** local session/UI state and preferences. Browser state is separate from PostgreSQL records.
- **OpenAPI client:** generated during the UI build from the bundled API specification. Its base URL is the console's origin; the gateway routes API requests.
- **Design system:** shared primitives under `ui/packages/design-system`, with console-specific Tailwind/CSS overrides.
- **Terminal:** xterm renders terminal interaction; actual authorization and session transport belong to the server/agent.

## CloudMateHub changes

`BrandLogo.tsx` draws the cloud/terminal mark and wordmark. `BrandStory.tsx` provides the welcome illustration and introduction. `AuthLayout.tsx` and `FirstRunLayout.tsx` provide responsive layouts; `SidebarShell.tsx` controls navigation sizing. `index.css` and `tailwind.config.js` define the teal/slate treatment. The SVG favicon is `public/cloudmatehub.svg`.

Visible copy uses CloudMateHub. Package imports, browser-storage keys, API routes and webhook protocol names retain compatibility. Existing built-in terminal-theme selections are normalized to the new display names. The upstream marketing survey is disabled through deployment configuration.

The UI does **not** talk directly to PostgreSQL or Valkey. Hiding a button is not authorization: backend checks remain the security boundary.

Related: [03 Server and Domain](03%20Server%20and%20Domain.md), [06 Data and Trust](06%20Data%20and%20Trust.md), [07 Development and Deployment](07%20Development%20and%20Deployment.md).

## Branding and compatibility

Owned UI copy, npm workspaces, icon symbols, theme assets and documentation use CloudMateHub. Published upstream links and legal attribution remain original. Persisted session/vault/browser-key identifiers retain their legacy values so existing accounts, preferences and encrypted vaults remain readable. The native SSH approval destination survives both initial sign-in and expired-token recovery.

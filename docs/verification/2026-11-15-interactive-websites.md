# Interactive website verification — v0.30.0

## Test data ownership

No persistent user, website, namespace, or device records are created. Backend tests use in-memory catalogs and disposable browser sessions. The worker integration test uses an isolated container, fake public-site responses, and ephemeral Unix sockets. Existing NVIDIA, Kladex and other catalog entries are preserved. Browser tests do not clear user storage.

## Checks

- Website package and application tests, including per-event authorization, API-key rejection, user/tab isolation, access revocation, private-address protection.
- Interactive worker tests with isolated page fixtures: navigation, typing, history, cookie isolation, attachment refusal, closed sessions.
- Console TypeScript/lint checks and production build.
- Live localhost UI navigation and input verification.

## Results (2026-11-16)

- PASS Go website and application packages, including session identity separation and revoked assignments.
- PASS isolated Chromium integration test: HTTP redirect, link navigation, typing, GET search, POST forms, history, cookie isolation, attachment and data-URL download blocking, session close.
- PASS console TypeScript and targeted ESLint checks (previous turn), production UI build and current local rollout.
- PASS live Hub UI: Varzesh3 redirects to its www host and opens a news article inside the Hub. Kladex form input and NVIDIA link navigation were also verified in the preceding turn.
- Public page captures: Example, IANA, Wikipedia and NVIDIA rendered and were visually inspected. MDN direct public-IP connections timed out from the workstation; local relay retest recorded below.
- Fixed browser process exhaustion with one Chromium process and isolated contexts, init reaping, bounded session count, and explicit resource limits.
- Fixed redirect interception and bounded frame capture during navigation. Public HTTP redirects keep their cookie and method semantics.

No persistent Hub fixtures were created; no database cleanup was needed. Temporary browser test sessions are closed in `finally`. Existing catalog entries were not edited. GitHub and Docker Hub were not updated.

Live verification session IDs are SHA-256 of `cmh-owned-verification-2026-11-16-` plus the case name (`Example`, `IANA`, `MDN`, `Wikipedia`, `NVIDIA`). The test wrote these exact IDs to its manifest before opening sessions. Test screenshots contain public website content only.


- PASS MDN retest through the existing local relay mechanism; homepage visually inspected. All five public pages loaded. NVIDIA and MDN use local relay egress; Example, IANA and Wikipedia use direct Hub egress.
- PASS HTTP 307 regression verifies POST method and form body survive redirects.
- Final worker usage after live tests: approximately 476 MiB and 150 processes within the 2 GiB / 768-process limits.
- The configured nested Obsidian vault path `../CloudMateHub/CloudMateHub` is absent on this workstation; matching architecture documentation was updated in `docs/architecture/11 Directory Authentication and Users.md`.

Exact owned live-test session IDs:

```json
{
  "Example": "0ef0050dbbbdb1201f955f445374ead1ade4d381d28b34169a6e08a42f045ea7",
  "IANA": "314235af3cd3e3865b5acce5e6f0d0fcff52fedd78b5f461896def86b8b695e5",
  "MDN": "96132392023dc9970ceb44dbfdeaf0a8d5e9bc9b5927a6ae08aab74b8fe4cb24",
  "Wikipedia": "84901ef769b59ec290daa2781a33d0e326323e36169a986befb870d1f2c5f957",
  "NVIDIA": "47ef9ac6a00f3a669316e5363472c12b84c51e1d7ee8ebd2b0f98cc0c825e63f"
}
```

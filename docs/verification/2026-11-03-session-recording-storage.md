# Session recording and storage verification

Date: 2026-11-03

## Delivered behavior

New SSH shell/exec seats are recorded at the shared server relay in Community builds, including native SSH, browser terminals, commands without a PTY, stdout and stderr. Namespace recording opt-out no longer disables this instance-wide policy. SFTP subsystem payloads and raw keystrokes are excluded; existing request/connection history remains. Programs can print secrets or binary data, which is still command output and can be captured.

Sessions and session detail pages play/download separate SSH channels. Instance administrators can select local disk, an already-mounted NFS share, or an HTTPS S3-compatible destination under Settings → Storage & Recordings. PostgreSQL retains the durable recovery copy of captured output. Storage failure does not block connections; archive writes retry. This is an availability-oriented recorder; the existing bounded event queue can log incomplete capture during database failure or exhaustion.

Configuration changes verify write/read/delete before saving and preserve older archive destination revisions. Secrets are encrypted in the new recording-state volume and omitted from API responses. The new recording-files volume is the active default. No external destination was configured on behalf of the user.

## Verification results

- Go checks passed for recordings, SSH session lifecycle, SSH channel forwarding, and server application wiring.
- Ten storage tests cover asciicast output/timing/resizing, multi-seat archives, paginated output, local/S3 roundtrip cleanup, encrypted credentials and redacted status, failed remote storage fallback, changing destinations/restarts, recording removal with history preserved, namespace/admin authorization, insecure credential rejection, active-session deletion rejection, nonblocking storage configuration during stalled archive work, symlink confinement before and after configuration, and refreshing snapshots after late output.
- S3 signing matches the complete AWS published GET-object Signature V4 test vector. S3 I/O was tested against an isolated TLS protocol fixture; no real S3 bucket or NFS server was supplied or tested.
- Console TypeScript checking and 84 targeted tests in eight files passed, including storage settings, SSH settings, connection dialog, sessions, multi-channel recording hooks and removal hooks. The final copy-only clarification identifies SFTP specifically.
- Production server and console images built and were activated. All eight deployment services were running after the update.
- Live dashboard health, the storage settings route, console entry asset and new lazy-loaded storage page asset returned HTTP 200.
- Live anonymous storage access, forged administrator headers and anonymous recording access were denied.
- The real recording-state directory is mode 0700; master.key and storage.enc are mode 0600.
- No authenticated live browser or new production SSH fixture walkthrough is claimed. The existing real sessions, accounts, device records and keys were preserved.
- An exploratory run of the broader service suite encountered its missing database integration environment in the build container. The focused feature/SSH/application checks above completed independently.

## Test ownership and cleanup

No persistent production test fixtures were created. Unit/protocol fixtures use isolated build-container filesystem state, fake database records, temporary TLS servers and explicit test-owned temporary directories with teardown. Their exact local paths are registered with test cleanup before use. Probe object ownership is registered before creation by the storage service; roundtrip checks verify removal. The live checks were read-only or rejected unauthenticated requests and created no records. No cleanup was needed on the deployed database. The real service encryption key/configuration and existing user data were preserved.

README and repository architecture notes were updated. The configured Obsidian vault ../CloudMateHub/CloudMateHub is absent from this checkout, so no matching vault files could be updated. Changes remain local and have not been pushed to GitHub.

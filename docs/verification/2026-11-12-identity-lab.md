# Identity lab verification

The workstation-only Compose project `cmh-identity-lab` owned all fixtures. Its isolated network connected OpenLDAP 1.5.0, Keycloak 26.7.5, the Hub server, PostgreSQL, Valkey, and Caddy. The HTTPS test client trusted only the lab CA. HTTP tests used isolated cookie jars.

## Fixture manifest

The fixture names and ownership were recorded before startup. Generated Hub IDs were captured before teardown.

| Fixture | Identifier |
| --- | --- |
| LDAP search account | `cn=reader,dc=identity,dc=test` |
| LDAP test users | `uid=alice,ou=people,dc=identity,dc=test`, `uid=bob,ou=people,dc=identity,dc=test`, `uid=outsider,ou=people,dc=identity,dc=test` |
| LDAP allowed group | `cn=hub-users,dc=identity,dc=test` |
| Keycloak | Realm `cmh-lab`, client `lab-mfa`, bootstrap user `lab-admin`, federated user `alice`, OTP credential labeled `Identity lab fixture` |
| Hub administrator | `lab-admin@identity.test`, `a4ae3d50-b3e0-478f-aeba-0c695fceaa54` |
| Initial Hub LDAP users | Alice `d34df3cd-c505-4e89-89f4-19cfcfa0bfca`, Bob `9a1c7f3d-8262-4e35-b53c-eb8e07abe0c6` |
| Reprovisioned Hub LDAP users | Alice `a741f79d-d24b-4627-aeca-8053b74b6fc3`, Bob `ed532488-e723-4952-8391-034cdce5078b` |
| Workspaces | `lab-alpha` `e7eceeff-0e22-4015-8788-043c88a380f1`, `lab-beta` `e8c6d7f8-9481-4902-a2e9-7eecfe56518f` |
| Websites | Alpha Docs `d39892864c3f58537a80904143cc80cb`, Beta Docs `faa5bc879f80d1f22b931424556aee59`, Directory Docs `dfcb24e443649b546ce08dc3aac4292d` |

## Results

- OpenLDAP bind, TLS trust, base search, seed data, allowed group, wrong password, outside-group user, and source throttling passed.
- Keycloak found the federated account over LDAPS. TOTP enrollment and fresh HTTPS login passed. Password-only, wrong-TOTP, and wrong-password token requests failed as expected. The correct factors issued a Keycloak token through the authorization-code flow.
- Hub directory dry runs left the user list unchanged. Saving settings redacted the bind secret. LDAP sign-in provisioned one non-admin account per identity. Non-admins were denied directory settings and instance-wide user and workspace rosters.
- The original Hub image returned HTTP 409 when the instance admin created a second workspace. Its system record was bound to the first workspace. The rebuilt server accepted the request with `CLOUDMATEHUB_MULTI_WORKSPACE=true` and retained the first workspace ID.
- The two LDAP members could obtain tokens only for their assigned workspace and could not read the other workspace roster or website links. Removing Alice's membership invalidated her old token and removed the workspace website from her view. Her LDAP account and directory-group website remained available.
- Focused Go tests passed for the directory, server startup, setup, namespace creation, and membership services. The website package compiled and its behavior passed through the lab API. A broader build-stage selector initially matched device-key integration tests that require PostgreSQL. The focused unit tests ran in the build stage, and the identity integration tests ran against the live lab database.

The LDAP lab needed a pre-generated DH parameter file to start LDAPS promptly and a scoped read ACL for the service bind. The Keycloak browser flow needed HTTPS for its authentication cookies. The test form parser used Keycloak's raw TOTP secret from the enrollment form to generate test codes. Keycloak MFA and the Hub's direct LDAP sign-in were verified independently. The Hub has no Keycloak OIDC sign-in integration.

## Cleanup

The initial LDAP fixtures had zero memberships and owned workspaces. The lab server's temporary directory settings were recreated during the upgrade, so those two fixtures were removed through the admin CLI and provisioned again. The Compose project was removed with its temporary storage and its test-only Hub image tag. The workstation's running Hub and its data were outside the lab.

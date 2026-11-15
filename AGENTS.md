# CloudMateHub project instructions

## Feature completion and test data

The user wants only **test/demo data** removed after each feature is added. Preserve real accounts, devices, namespaces, sessions, settings, signing keys, and user files. Never interpret this as permission to reset the entire database or use `docker compose down -v`.

Before creating persistent fixtures, record their exact identifiers and test ownership in the feature's test-data manifest or cleanup log. After verification, remove only those explicitly owned fixture records through the relevant supported API or fixture teardown. Inspect dependencies before removing a namespace/user to avoid cascading into real records. A name containing `test` or `demo` is not proof of ownership.

If no persistent fixtures were created, record that no cleanup was needed. If ownership is ambiguous, preserve the records and report the ambiguity rather than deleting them. Browser test state must be isolated from real user sessions. Never clear a user's browser storage globally.

## Documentation

Update README.md and the matching Obsidian architecture notes when a feature changes architecture, deployment, or data flows. The vault is `../CloudMateHub/CloudMateHub`. Keep secrets out of notes, diagrams, screenshots and Git. Preserve upstream licensing and source attribution.

## Workstation Docker and What's New

After implementing and verifying feature changes, build and deploy the latest changes to the workstation Docker stack for review. Use versioned images and preserve existing volumes and signing keys. GitHub pushes and Docker Hub publication still require the user's approval.

For every major user-facing update, update the console's What's New catalog in `ui/apps/console/src/components/layout/updateTutorial.ts`. Give the update a new ID so users see it once, describe the new workflow in short steps, and gate admin-only steps by role. Verify the tutorial before the workstation rollout.

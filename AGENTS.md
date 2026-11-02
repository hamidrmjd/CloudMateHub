# CloudMateHub project instructions

## Feature completion and test data

The user wants only **test/demo data** removed after each feature is added. Preserve real accounts, devices, namespaces, sessions, settings, signing keys, and user files. Never interpret this as permission to reset the entire database or use `docker compose down -v`.

Before creating persistent fixtures, record their exact identifiers and test ownership in the feature's test-data manifest or cleanup log. After verification, remove only those explicitly owned fixture records through the relevant supported API or fixture teardown. Inspect dependencies before removing a namespace/user to avoid cascading into real records. A name containing `test` or `demo` is not proof of ownership.

If no persistent fixtures were created, record that no cleanup was needed. If ownership is ambiguous, preserve the records and report the ambiguity rather than deleting them. Browser test state must be isolated from real user sessions. Never clear a user's browser storage globally.

## Documentation

Update README.md and the matching Obsidian architecture notes when a feature changes architecture, deployment, or data flows. The vault is `../CloudMateHub/CloudMateHub`. Keep secrets out of notes, diagrams, screenshots and Git. Preserve upstream licensing and source attribution.

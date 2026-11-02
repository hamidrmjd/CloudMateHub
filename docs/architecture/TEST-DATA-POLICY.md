# Test and demo data policy

After every feature, remove only the persistent test/demo records that the feature created and explicitly owns. Real user data remains intact.

1. Record fixture identifiers and environment before creating them.
2. Run the feature checks against isolated fixtures.
3. Inspect dependent records and remove owned fixtures through supported teardown/API paths.
4. Record cleanup results. If no persistent fixtures were created, state that no cleanup was needed.

Do not infer ownership from names, delete unlabelled records, wipe the database or its Docker volumes, rotate signing keys, or clear the user's browser storage. When ownership cannot be established, preserve the data.

For this documentation change, no application test/demo records were created or deleted. Existing accounts/devices were not inspected or reset.

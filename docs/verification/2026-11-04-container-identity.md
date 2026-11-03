# CloudMateHub container identity migration

The active Compose project is cloudmatehub. Server, console and gateway were built from local repository source as cloudmatehub/server:local, cloudmatehub/ui:local and cloudmatehub/gateway:local. Base Compose application image references no longer pull ShellHub application images.

A private database backup was saved before switching. All ten existing persistent data volumes and signing-key mounts were reused. The existing application network is reused as external so connected demo agents can reconnect. Legacy storage/network names remain migration compatibility identifiers, not a dependency on running legacy containers. Standard PostgreSQL, Valkey, WGDashboard and build-tool dependencies retain their original provenance and licensing.

All eight replacement services passed health checks; public API health returned 200. Only the eight stopped, project-label-verified shellhub-custom containers were removed, without deleting volumes. No test fixtures were created or removed; previously requested demo devices remain retained. README, deployment notes and repository architecture notes were updated. The separate Obsidian vault path is absent.

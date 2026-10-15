# Compatibility Go wrapper

Reviewed source snapshot, retaining `github.com/meshory/scan-fs` module identity and the original MIT notice. This is a compatibility CLI, not a newly supported Go SDK.

From Scanner root run `python3 scripts/build-components.py wrapper` to build the local engine and embed the matching payload before compiling `scan-fs`. No private release downloads or `latest` resolution occur.
`SCAN_ENGINE_PATH` still overrides the embedded engine. `SCAN_FS_SOURCE`, flags and flat JSON normalization retain their earlier behavior.

The project catalog also has a `scan-fs` command. These binaries ship in separate archives; install one component per directory. See [compatibility](../../docs/consolidation/compatibility.md).

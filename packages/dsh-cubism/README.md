# dsh-cubism (project-local prototype)

Eight DSH agent tools for **this repository's existing localhost studio API**, plus two Windows-only fixture rigging tools. Inspired by the installed `@opdsh/unity-plugin` tool-registration architecture, not a port of its unrestricted Unity C# evaluation.

This directory is source-only ESM (no build step). Merely adding it to the repository **does not install or enable it in any DSH profile**. Profile installation/activation requires the parent/user's separate authorization; none was performed for development or tests. It is marked private to prevent accidental npm publication.

## Actual compatibility and source evidence

Read-only inspection found `@opdsh/unity-plugin` **0.1.7** in `C:/Users/13929/.dsh/profiles/web/package.json`. Its installed package metadata, `cordis.patch.yml`, and bundled entry establish the real `dsh.bundle.patch` manifest, named `name`/`inject`/`Config`/`apply` exports, Schemastery configuration and `ctx.tools.register(defineTool(...))` architecture. Public sources: [Unity manifest](https://github.com/opdsh/unity-plugin/blob/main/package.json), [Unity entry](https://github.com/opdsh/unity-plugin/blob/main/src/index.ts). Installed artifacts, rather than moving public `main`, were the compatibility authority.

The local published DSH bundle at `C:/Users/13929/AppData/Roaming/npm/node_modules/@deepseek-ai/dsh` is **0.2.2-rc.1**, with `@deepseek-ai/dsh-tools` **0.2.2-rc.1** and Schemastery **3.19.x**. This package pins that tools peer and the observed Schemastery minor; other DSH releases are not claimed compatible. Requires Node **>=24.9.0** (tested with 24.22.2). It uses only the `tools` injection; each fixture runner invokes its fixed package-local Windows PowerShell script.

The read-only SDK probe loads the real installed exports, parses configuration through real Schemastery and registers all ten definitions through real `defineTool`. It does **not** prove profile-loader activation or a successful Cubism operation. No live profile was modified or Editor command issued during testing.

## Configuration

```yaml
# Config values for the cubism row if an authorized profile later composes it:
studioUrl: http://127.0.0.1:17823
timeoutMs: 30000
```

Studio origin is deployment configuration, not an agent tool argument. Only HTTP loopback origins with an explicit non-default port are accepted; paths, credentials, query and fragment are rejected. IPv4 loopback, IPv6 `::1` and `localhost` work. Redirects are refused, JSON responses capped at 1 MiB and caller cancellation is forwarded. Requests have no automatic retry: a timeout/failure may occur after a mutation takes effect. Error response bodies are deliberately not echoed. Requests carry no credentials, and there are no tools to read/write studio config, read registration tokens, or invoke paid generation endpoints. Loopback HTTP is not a substitute for trusted local-machine access control.

The default studio address is the default in [the studio command](../../cmd/studio/main.go). Start the studio separately using the repository's documented launch command; the plugin does not start it. Configuring a Cubism executable remains a studio UI/server responsibility. Restart/reload the plugin to change configuration; no volatile settings-page integration is supplied.

## Tools and exact studio routes

| Tool | Request | What success means |
| --- | --- | --- |
| `cubism_health` | GET `/api/health` | Studio reachable, not Editor ready |
| `cubism_discover` | POST `/api/l2d/cubism/discover` `{}` | Candidate discovery only, no selection/config change |
| `cubism_launch` | POST `/api/l2d/cubism/launch` `{}` | Launch requested; license/readiness/model unverified |
| `cubism_connect` | POST `/api/l2d/cubism/connect` `{endpoint?}` | Inspect returned `connected` and `approved`; approve access in Cubism |
| `cubism_command` | POST `/api/l2d/cubism/command` `{method,data}` | Official supported command result, not inferred task completion |
| `cubism_disconnect` | POST `/api/l2d/cubism/disconnect` `{}` | Bridge closed, Editor not quit |
| `cubism_export_psd` | POST `/api/oc/export-psd` `{width,height,layers}` | Local layered PSD exported, no model/mesh/rig created |
| `cubism_open_psd` | POST `/api/l2d/cubism/open-psd` `{exportId}` | Document launcher invoked; import/model creation unverified |

Contracts: [automation routes](../../CoreServer/cubism_automation.go), [PSD workflow routes](../../CoreServer/assets_workflow.go), [discovery/launch/health routes](../../CoreServer/server.go), [official bridge allowlist](../../l2d/cubism_api.go), [PSD writer](../../oc/psd.go).

Enable Cubism Editor's external API and approve plugin access in its UI. Default endpoint `ws://127.0.0.1:22033`; optional overrides must be loopback `ws` with explicit port and no credentials/query/fragment. The studio alone owns the Cubism registration token. HTTP 200 does not imply connected or approved. Suggested flow: health → discover → configure executable in studio if needed → launch → enable external API/approve → connect → GetDocuments/GetCurrentModelUID → model-specific reads/writes.

### Official command subset

`data` uses **PascalCase**, not camelCase. Omitted `data` becomes `{}`.

- `GetDocuments`, `GetCurrentModelUID`: empty data only.
- `GetParameters`: `ModelUID` or `DocumentUID`.
- `GetParameterValues`: `ModelUID`, optional `Ids` array (at most 1024).
- `SetParameterValues`: `ModelUID`, `Parameters: [{Id,Value}]` (1..1024 unique IDs, finite values within ±1000000).
- `ClearParameterValues`: `ModelUID`.

Example tool arguments:

```json
{"method":"SetParameterValues","data":{"ModelUID":"<UID returned by Cubism>","Parameters":[{"Id":"ParamAngleX","Value":10}]}}
```

The official bridge does not expose arbitrary WebSocket envelopes, registration, eval, mesh editing, deformer/rig creation, model saving or `.moc3` export. Parameter changes are not a rigging API and persistence is not promised. Unknown method-specific fields fail closed locally and the server independently enforces its allowlist.

### PSD workflow

Use existing studio asset IDs (24 lowercase hex characters). Canvas is 1..8192 per axis, at most 40 megapixels; 1..64 layers; signed offsets within ±8192. Each layer accepts `assetId`, optional `name`, `group`, `x`, `y`, `width`, `height`. Both resize dimensions zero/omitted preserve source size, otherwise both must be positive within the same size limits. The server additionally enforces source validity, combined 80-megapixel layer budget and output byte budget.

First layer is topmost. Adjacent equal groups form PSD folders; they do not set Cubism numeric draw-order properties. Export uses local assets and no paid provider call. Export response includes `id` and relative download `url`; resolve that URL against the configured studio origin. `cubism_open_psd` accepts only that export ID, not arbitrary file paths. Confirm Model Settings in Cubism. If no dialog appears, download the PSD and use **File > Open**. Import and model creation remain unverified.

### Windows fixture eye rig

`cubism_build_eye_rig` takes `{}` and runs only on an explicit user request. Unlike `cubism_command`, it uses the local Cubism FREE desktop, not the official External API. Start the Editor and make the imported test PSD `8b2b9c88e31a0519f6fe2cf3` active first; arbitrary documents, scripts and paths are not supported. Leave the desktop untouched while it runs.

The tool runs only the bundled [`build-eye-rig.ps1`](scripts/build-eye-rig.ps1) with a fixed absolute `-OutputDirectory` (`.local/qa/one-click-rig` in the source workspace). The tool is intended to run from this repository's package location. It allows 90 seconds, forwards cancellation, uses an exclusive workspace lock, and doesn't retry. Successful output is `{status:"saved",modelPath,screenshots}` after confirming the reported model and screenshot files are nonempty and inside the output directory. A failure or cancellation may leave edits in the Editor: inspect it before retrying. A host crash can leave a lock; only remove it after verifying that no recipe process is running.

### Windows deterministic face fixture rig

`cubism_build_face_rig` also takes `{}` and requires an explicit user request. It is only for the deterministic layered face test fixture imported and active in Cubism FREE, **not arbitrary character rigging**. It invokes only [`build-face-rig.ps1`](scripts/build-face-rig.ps1), with fixed output `.local/qa/face-rig`. Generate the isolated PSD with `go run scripts/make-face-fixture.go` from the workspace root and import `face-fixture.psd` as a new model. The recipe provides independent anatomical `ParamEyeLOpen`/`ParamEyeROpen` keyforms through upper eyelid translation, plus `ParamMouthOpenY` through a separate upper-mouth skin cover. Iris, sclera, cavity and tongue are independently drawn and remain unscaled. The tongue has its own mesh translation keyforms linked to `ParamMouthOpenY` (recedes on closure); there is no independent tongue-out/curl parameter yet. This is an occlusion/translation sample, not production-quality curved eyelid or lip deformation. Actual FREE 5.3.05 save/reopen acceptance is recorded in the project acceptance document. Do not touch the desktop while either recipe runs.

Both tools share `.local/qa/.cubism-rig.lock`, so eye and face recipes cannot control the desktop simultaneously. The face tool inherits the same 90-second timeout, cancellation, no-retry policy, output containment/nonempty-file checks and sanitized failure handling. These checks validate saved files, not visual rig correctness. Tests inject subprocess stubs and never mutate the desktop. No profile reload or installation is part of this change.

## Validation commands (no dependency/profile installation)

From this package directory:

```bash
npm test
npm run check:sdk -- 'C:/Users/13929/AppData/Roaming/npm/node_modules/@deepseek-ai/dsh'
```

The first command uses Node built-ins only: mocked route/body tests plus an ephemeral real loopback HTTP server, cancellation, timeout, redirects, HTTP/error privacy, response bounds, command allowlist and PSD validation. The second uses Node's synchronous module resolution hook solely inside the probe process to resolve the plugin's two peers from an explicit existing DSH directory. It writes no symlinks, dependencies, profile config or global state. Production entry imports normal peers; this test hook is not a runtime fallback.

To inspect a distributable file list without installing or enabling anything:

```bash
npm pack --dry-run
```

Actual profile install/enable and end-to-end Editor verification are intentionally pending explicit parent/user authorization and a running approved Cubism Editor.

---
owner: @SivanCola
backup: @esengine
status: active
reviewed: 2026-10-02
---

# Starter Extension

This directory is a complete, installable Extension Protocol v2 plugin. Its
sidecar intercepts `input.receive`; while enabled, it appends
` [rewritten by starter-extension]` to every non-empty input payload.
It preserves the complete text, including any context the host has composed
before calling the interceptor.

The `.exe` suffix is intentional: using one fixed runtime path keeps the
manifest identical on every platform. Unix executes the binary normally, and
Windows requires the executable suffix.

## Build and install

From this directory on macOS or Linux:

```sh
go build -o bin/starter-extension.exe .
plugin_root="$(pwd -P)"
reasonix plugin install "$plugin_root" --dry-run
reasonix plugin install "$plugin_root" --link --replace --yes
```

From PowerShell on Windows:

```powershell
go build -o bin/starter-extension.exe .
$pluginRoot = (Resolve-Path .).Path
reasonix plugin install $pluginRoot --dry-run
reasonix plugin install $pluginRoot --link --replace --yes
```

Review the `FULL TRUST` block in the dry-run output before installing. The
linked package trusts future changes in this directory and runs outside the
Reasonix sandbox.

Start a new session, or run `/reload` while the current session is idle. Send:

```text
explain what an Extension Protocol sidecar does
```

The model receives the text with the marker appended; the user bubble keeps
your original input. A live model's reply is not an exact trace of its input.
Use the host check below to verify the text that reaches the provider.

Edit `main.go`, rebuild the binary, run `/reload`, and try again. Use
`reasonix plugin doctor starter-extension` when the manifest or binary fails
validation.

To end the demo, disable or remove it and start a new session or reload while
idle:

```sh
reasonix plugin disable starter-extension
reasonix plugin remove starter-extension --yes
```

Removing a linked installation leaves this source directory in place.

## Run the host check

From the repository root:

```sh
go test ./internal/assembly/boot/ -run '^TestEffectStarterExtensionComposedInput$' -count=1
```

The check builds this actual SDK example without fetching modules, previews
and applies a copy installation in an isolated home, then removes the source.

It submits turns through the controller with a response-language preference
and asserts the composed context, original input, and marker at a recording
provider.

It covers absent, installed, disabled, reenabled, and removed states and waits
for each started sidecar to exit. It does not test a live model's reply or
Studio's rendering.

## Next steps

- [`../../README.md`](../../README.md) documents SDK callbacks and the
  concurrency contract.
- [`../../../../docs/EXTENSIONS.md`](../../../../docs/EXTENSIONS.md) explains
  reload, performance, cache behavior, compatibility, and trust.
- [`../../../../docs/PLUGIN_PACKAGES.md`](../../../../docs/PLUGIN_PACKAGES.md)
  defines every Manifest v2 field.
- [`../../../../docs/EXTENSION_PROTOCOL.md`](../../../../docs/EXTENSION_PROTOCOL.md)
  is the wire-protocol reference.
- [`../fullsidecar/main.go`](../fullsidecar/main.go) demonstrates providers,
  structured UI, strategies, tools, content references, and shutdown.

For a distributable plugin, build binaries for the target platforms, keep the
manifest runtime path aligned with the packaged binary, and publish immutable
source or release artifacts for users to review before installation.

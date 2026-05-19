# macOS code-signing & notarization setup

Until this is wired up, customers installing toolkit on macOS hit Gatekeeper's *"toolkit cannot be opened"* dialog on first run, because the released tarballs are unsigned. The current tap README documents a `xattr -dr com.apple.quarantine` workaround, which works but is a poor onboarding experience.

This doc covers the manual one-time setup. Once complete, every release that follows passes Apple's notary service automatically — and Gatekeeper no longer quarantines the binary.

## How the pipeline behaves

The release workflow's `.goreleaser.yaml` already contains a `notarize.macos[]` block (gated on env-var presence). The behavior is:

- **All five env vars set** → GoReleaser invokes `quill` (Anchore's notarytool wrapper) which (a) code-signs every darwin binary with the Developer ID, then (b) submits to Apple's notary service and waits for the verdict before publishing the release.
- **Any env var missing** → the `notarize` pipe is skipped (`reason=disabled`). Release still publishes; binaries are unsigned (current behavior).

The CI snapshot smoke-test (`ci.yml: release-snapshot`) always runs without these secrets, so PRs continue to validate the build pipeline without needing notarization credentials.

## One-time manual setup checklist

### 1. Developer ID Application certificate

- [ ] Enroll in the [Apple Developer Program](https://developer.apple.com/programs/) ($99/year) if not already.
- [ ] In *Certificates, IDs & Profiles*, create a new **Developer ID Application** certificate (not "Developer ID Installer" — that's for `.pkg` installers, not binaries).
- [ ] Download the certificate, double-click to add to Keychain Access.
- [ ] In Keychain, find the new certificate, expand it to reveal the matching private key, right-click → **Export 2 items**. Save as `developer-id.p12` with a strong export password.
- [ ] Base64-encode the export:
  ```bash
  base64 -i developer-id.p12 -o developer-id.p12.b64
  ```

### 2. App Store Connect API key (for notarytool)

- [ ] In [App Store Connect → Users and Access → Integrations → App Store Connect API](https://appstoreconnect.apple.com/access/integrations/api), create a key with **Developer** role.
- [ ] Note the **Key ID** (10 chars, e.g. `ABC1234DEF`) and **Issuer ID** (UUID, e.g. `69a6de7f-...`).
- [ ] Download the `.p8` private key (**Apple only lets you download it once** — save it before closing the dialog).
- [ ] Base64-encode the key:
  ```bash
  base64 -i AuthKey_ABC1234DEF.p8 -o AuthKey.p8.b64
  ```

### 3. GitHub repository secrets

Open https://github.com/jingle2008/toolkit/settings/secrets/actions and add five new secrets:

| Secret name | Value |
|---|---|
| `MACOS_SIGN_P12_BASE64` | contents of `developer-id.p12.b64` |
| `MACOS_SIGN_PASSWORD` | the `.p12` export password from step 1 |
| `MACOS_NOTARY_KEY` | contents of `AuthKey.p8.b64` |
| `MACOS_NOTARY_KEY_ID` | the 10-char Key ID from step 2 |
| `MACOS_NOTARY_ISSUER_ID` | the UUID Issuer ID from step 2 |

### 4. Verify

- [ ] Cut a patch release (`v0.3.1` or similar) by pushing a new tag. Watch the Release workflow.
- [ ] In the GoReleaser step's log, look for `sign & notarize macOS binaries` followed by per-binary `quill: signed` and `quill: notarized accepted` messages (rather than `pipe skipped or partially skipped reason=disabled`).
- [ ] On a Mac (clean, never-ran-toolkit), download the new darwin tarball from the release page, extract, run `./toolkit version`. Should print the version without a Gatekeeper prompt.
- [ ] Optionally verify the binary's stapled notarization:
  ```bash
  spctl -a -vv -t install ./toolkit
  # → ./toolkit: accepted (source=Notarized Developer ID)
  ```

## What didn't change

- The release workflow still runs on `ubuntu-latest`. Quill is cross-platform; the `codesign` binary is *not* used — Quill re-implements the relevant parts of the macOS code-signing protocol in Go so notarization works without a macOS runner.
- The Homebrew Cask path is unchanged. Once notarized binaries ship, Gatekeeper accepts them on first launch and the Cask's `xattr` workaround in the tap README can be removed.

## Operational notes

- Apple's notary service has been known to take 1–20 minutes per submission. The `wait: true` / `timeout: 20m` in `.goreleaser.yaml` reflects this — GoReleaser polls until done. If a release stalls in the notarize step, check [Apple System Status](https://developer.apple.com/system-status/) for notary outages.
- Developer ID certificates expire every 5 years. App Store Connect API keys don't expire unless revoked. Refresh secrets accordingly.
- Quill version is pinned by goreleaser. If signing starts failing after a goreleaser upgrade, check the goreleaser release notes for quill bumps and confirm the cert format hasn't drifted.

# Plugin Registry Authoring Guide

This document explains how to create and publish a Songloft plugin registry, so that other users can subscribe to your registry URL and browse and install your plugins from the in-app "Plugin Store".

---

## What Is a Plugin Registry

A plugin registry is a JSON file containing a list of `plugin.json` URLs. Once a user adds your JSON URL under "Settings → JS Plugin Management → Plugin Store → Manage Subscriptions", they can see all the plugins in your registry and install them with one click.

The store automatically fetches each plugin's name, version, description, and other metadata from its `plugin.json` URL, so there's no need to duplicate that information in the registry file.

Plugin registries support **nested references** (`includes`), letting you compose multiple standalone registries for decentralized plugin distribution.

---

## JSON Format Specification

```json
{
  "name": "My Plugin Registry",
  "includes": [
    "https://example.com/other-registry.json"
  ],
  "plugins": [
    "https://raw.githubusercontent.com/you/example-plugin/main/plugin.json",
    "https://raw.githubusercontent.com/you/another-plugin/main/plugin.json"
  ]
}
```

### Field Reference

| Field | Type | Required | Description |
|------|------|------|------|
| `name` | string | No | Registry name, shown in the UI |
| `includes` | string[] | No | Array of nested registry URLs to reference |
| `plugins` | string[] | Yes | Array of URLs to each plugin's `plugin.json` |

### Automatic Resolution

Each URL in `plugins` points to a `plugin.json` in a plugin repository. The store automatically fetches and parses the following fields:

- `name` — plugin name
- `entryPath` — unique plugin identifier
- `version` — version number
- `description` — description
- `author` — author
- `homepage` — project homepage
- `download_url` — ZIP package download URL
- `updateUrl` — update check URL
- `minHostVersion` — minimum host version

If `plugin.json` does not contain a `download_url` (which is the case for most plugins), the store automatically retrieves it from the `manifest.json` referenced by `updateUrl`.

---

## Complete Examples

### Minimal Example

```json
{
  "plugins": [
    "https://raw.githubusercontent.com/you/my-plugin/main/plugin.json"
  ]
}
```

### Full Example with Nesting

```json
{
  "name": "Songloft Community Aggregate Registry",
  "includes": [
    "https://raw.githubusercontent.com/alice/songloft-plugins/main/registry.json",
    "https://raw.githubusercontent.com/bob/my-plugin-registry/main/registry.json"
  ],
  "plugins": [
    "https://raw.githubusercontent.com/songloft-org/songloft-plugin-miot/main/plugin.json",
    "https://raw.githubusercontent.com/songloft-org/songloft-plugin-subsonic/main/plugin.json"
  ]
}
```

---

## Publishing to GitHub

### Option 1: In-Repo Raw URL (Recommended)

1. Create `registry.json` in the root of your GitHub repository
2. Push to the `main` branch
3. The registry URL is:
   ```
   https://raw.githubusercontent.com/{username}/{repo}/main/registry.json
   ```

**Hosting the plugin ZIP**: GitHub Releases are recommended:
1. Create a Release in the plugin repository
2. Upload the `.jsplugin.zip` as a Release Asset
3. In `plugin.json`, point `updateUrl` at `manifest.json`, and set `download_url` in `manifest.json`:
   ```
   https://github.com/{username}/{repo}/releases/download/v{version}/{entry_path}.jsplugin.zip
   ```

### Option 2: GitHub Pages

1. Enable GitHub Pages for the repository
2. Place `registry.json` at the Pages root
3. The registry URL is:
   ```
   https://{username}.github.io/{repo}/registry.json
   ```

### Example Repository Structure

```
my-plugin-registry/
├── registry.json          ← Registry JSON (just lists plugin.json URLs)
└── README.md
```

The plugins themselves are maintained in their own repositories, for example:

```
my-songloft-plugin/
├── plugin.json            ← Plugin metadata (name, version, entryPath, etc.)
├── manifest.json          ← Update manifest (version + download_url)
├── main.js                ← Plugin entry point
└── ...
```

---

## Publishing to a JS CDN

### jsDelivr (via npm)

1. Create an npm package containing `registry.json`
2. Publish to npm:
   ```bash
   npm publish
   ```
3. The registry URL is:
   ```
   https://cdn.jsdelivr.net/npm/{package}@latest/registry.json
   ```

### unpkg (via npm)

Same as jsDelivr, just swap the domain:
```
https://unpkg.com/{package}@latest/registry.json
```

### jsDelivr (via GitHub)

You can also use jsDelivr to accelerate GitHub files directly without publishing an npm package:
```
https://cdn.jsdelivr.net/gh/{username}/{repo}@{branch}/registry.json
```

---

## Use Cases for Nested Registries

The `includes` field lets one registry reference others, recursively downloading and merging all their plugins.

### Typical Patterns

**Aggregator registry**: collect several independent authors' registries so users can subscribe to them all at once:
```json
{
  "name": "Community Aggregate",
  "includes": [
    "https://raw.githubusercontent.com/alice/plugins/main/registry.json",
    "https://raw.githubusercontent.com/bob/plugins/main/registry.json",
    "https://raw.githubusercontent.com/charlie/plugins/main/registry.json"
  ],
  "plugins": []
}
```

**Official + community**: an official registry with core plugins that also pulls in community contributions:
```json
{
  "name": "Official Registry",
  "includes": [
    "https://community.example.com/registry.json"
  ],
  "plugins": [
    "https://raw.githubusercontent.com/songloft-org/songloft-plugin-miot/main/plugin.json"
  ]
}
```

### Deduplication Rules

Deduplication keys on **plugin identity**, not on `entryPath` alone:

- **Identity = `entryPath` + author.** The author comes from the `author` field in `plugin.json` and is normalized before comparison (case-insensitive, with `<email>` and `(notes)` stripped), so `hanxi` and `Hanxi <a@b.com>` count as the same person.
- When `author` is empty, identity falls back to the GitHub repository (`owner/repo`) that `updateUrl` points at. Self-hosted URLs are not inferred — their path layout is arbitrary, so repository ownership cannot be derived reliably.
- When neither yields an identity, deduplication falls back to `entryPath` alone.

Consequently:

| Case | Result |
|------|--------|
| The same plugin listed in several registries (including nested ones) | Merged into one entry, keeping the **higher version** (versions are compared segment by segment numerically after splitting on `.`) |
| Two plugins sharing an `entryPath` but from **different authors** | **Each gets its own entry**, both visible and installable in the store |

> **Two plugins with the same `entryPath` cannot coexist locally.** `entryPath` is simultaneously the plugin's install directory name, route prefix, and data ownership key, so only one plugin per `entryPath` can be installed. If the plugin you want collides with an installed one, the store marks that entry as conflicting and names the plugin occupying the identifier; choosing "Replace" and confirming swaps the original out, and **the replacement inherits the original plugin's stored data**. To spare your users this dilemma, pick a sufficiently distinctive `entryPath` when publishing a plugin.

---

## Progressive Client Loading

Flutter and Lynx fetch registries, recursive `includes` and plugin metadata from the client, updating the list as each plugin resolves rather than waiting for slower requests. At most eight requests run concurrently. Once the initial catalog completes, pages contain 20 entries, with search and pagination reusing a five-minute session cache. Explicit refresh reads the source configuration again and bypasses the cache; changing source or search cancels updates from the old listing.

If refresh finds the selected source removed or disabled, it falls back to all enabled sources. Valid empty catalogs replace the old cache, so later searches do not resurrect deleted entries. After installing an early result, a later higher version still offers an update.

Source settings and installed plugins still come from Songloft; installation and updates still run on the server. Web falls back to the existing server reader on CORS or connection failures. Native clients retain successful entries and report failed requests as warnings. The first result can usually appear earlier, while total time depends on client connectivity, source responses and whether Web needs fallback.

## GitHub Discovery Icons

Set `icon` in the root `plugin.json` on the default branch to show the icon in the GitHub discovery list and details. A bare filename such as `icon.svg` resolves under `static/`. A path such as `frontend/public/icon.svg` resolves from the repository root. Clients support SVG and bitmap formats they can decode. Missing or unavailable icons fall back to the first character of the plugin name.

## GitHub Discovery Token

Flutter and Lynx provide a GitHub Token entry at the top of the GitHub discovery page, with save, replace, and clear actions. The token is stored on the connected Songloft server and read when the client enters the page or explicitly refreshes it. Discovery requests still originate from the client.

- Settings endpoint: authenticated clients read and write `{ "token": "..." }` through `GET/PUT /api/v1/settings/github-discovery-token`. An unset token is empty; PUT with an empty string clears it. Responses must not be cached.
- With a token, search and Release queries connect directly to `https://api.github.com` and authenticate through the Authorization header. Neither GitHub acceleration proxies nor public plugin metadata requests receive this token. Failed authenticated requests do not fall back to an acceleration proxy.
- Without a token, discovery keeps its existing GitHub proxy and direct fallback behavior. Saving or clearing refreshes discovery results; invalid or expired tokens produce a specific message.

This setting is separate from private registry tokens described below, and **the source's own token takes precedence**. When the store client requests the official GitHub API, it only uses the shared token if the request has no source token. The shared token is not used for raw metadata, third-party registries, server fallback or installation downloads. Once configured, the client must be able to access the official GitHub API.

## Private Registry Authentication

Plugin registries support **Bearer Token authentication**, used to distribute closed-source plugins or to access plugins in private GitHub repositories.

### How to Configure

When adding or editing a registry in the "Manage Subscriptions" dialog, simply fill in the Token field. Registries with a configured Token show a lock icon in the list.

The token is sent as an `Authorization: Bearer <token>` header. Scoping rules:

- **The registry JSON itself**, **all URLs in the plugins array**, and **ZIP downloads** — always carry the token
- **Sub-registries referenced by includes** — clients inherit the token only on the **same origin** (scheme, host and port); the compatible server reader checks the same host. Cross-origin includes do not inherit it

Source tokens take precedence over the shared GitHub token. Authenticated metadata requests, server fallback reads and installation downloads bypass GitHub acceleration proxies; neither credential is sent to a prefix proxy. Clients only inherit source tokens for chained `updateUrl` metadata on the origin of the original manifest or configured source.

As a result, a private registry can safely include public official or third-party registries without leaking the token to them. Meanwhile, private sub-registries on the same server (same-host includes) can still authenticate normally.

### Private GitHub Repositories

1. Create a [Fine-grained Personal Access Token](https://github.com/settings/tokens?type=beta) on GitHub
2. Permissions: only **Contents: Read-only** on the target repository is needed
3. Use the `raw.githubusercontent.com` format for the registry URL, same as public repositories:
   ```
   https://raw.githubusercontent.com/{username}/{private-repo}/main/registry.json
   ```
4. Fill the PAT into the Token field under "Manage Subscriptions"

> A GitHub PAT works for both `raw.githubusercontent.com` (registry/plugin.json) and `github.com` (Release downloads). The Go HTTP client automatically strips the Authorization header on cross-host redirects, and the S3 signed URL that Release downloads redirect to does not require a token, so the behavior is correct.

### Self-Hosted Private Registries

Simply validate the `Authorization: Bearer <token>` header on your server. Using nginx as an example:

```nginx
location /registry/ {
    if ($http_authorization != "Bearer your-secret-token") {
        return 401;
    }
    root /var/www/plugins;
}
```

### Security Considerations

- The token is stored in plaintext in the server's config table (this is a self-hosted app, within the user's control)
- Clients only inherit tokens for same-origin `includes`; the compatible server reader checks the same host. Cross-origin includes do not inherit tokens
- The token is sent via the `Authorization` request header and never appears in the URL, so it won't leak into logs or the Referrer

---

## Notes and Limits

| Limit | Value | Description |
|------|-----|------|
| Maximum plugins per source | 500 | Excess entries are truncated and logged as a warning |
| Maximum client registries | 512 per source | Excess registries are skipped with a warning |
| Maximum recursion depth | 20 levels | `includes` beyond 20 levels are skipped |
| Maximum size per JSON | 2 MB | Larger files are rejected |
| Request timeout per URL | 15 seconds | On timeout, the URL is skipped (logged as a warning) |
| Circular references | Auto-detected | A → B → A won't loop forever; already-visited URLs are automatically skipped |
| Result cache | 5 minutes | The client caches the complete catalog; server fallback is also cached, so search and pagination do not re-fetch remote data |

- URLs in `plugins` must point to valid `plugin.json` files
- **Remember to hit "Refresh" while iterating on a registry**: store paging and search both operate on the
  complete cached list, so after editing your registry files plain paging won't show the change.
  The refresh button bypasses the cache and re-fetches (the cache also expires on its own after 5 minutes).
  Plugin install state is unaffected by the cache and is always computed live
- Anonymous GitHub requests can use the configured GitHub mirror accelerator; authenticated requests go directly to the source. Client requests use the device's network. The server's HTTP proxy only affects server fallback reads and installation downloads, not client request routing
- A failed fetch of one `includes` or `plugins` entry does not affect the loading of other registries and plugins

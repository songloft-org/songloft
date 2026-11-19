# Landing Page Screenshot Update Guide

The product UI screenshots shown on the documentation site's home page (landing page) are captured by driving the system Chrome with [Playwright](https://playwright.dev/), automatically logging into a locally running Songloft instance. The script is [`docs/scripts/capture-screenshots.mjs`](https://github.com/songloft-org/songloft/blob/main/docs/scripts/capture-screenshots.mjs), and its output lands in `docs/public/screenshots/`, committed alongside the repository.

> When you need to update the UI screenshots (e.g., after a UI redesign), just re-run the script following this guide — no manual screenshotting required.

## Prerequisites

1. **Run a Songloft service locally**, listening on the default port `58091`, with the default credentials `admin / admin`:

   ```bash
   make run          # or start the full build any way you like (requires the bundled web frontend)
   ```

   > The screenshots capture the **Flutter Web frontend** bundled in the full build. The lite build has no frontend and cannot be used for screenshots.
   > It's recommended to have several songs and playlists with covers in your library so the screenshots look good (the script captures the home page playlists, song library, player, etc.).

2. **Install dependencies** (in the `docs/` directory, only once; `playwright` is already in `docs/package.json`). The script uses the Google Chrome already installed on your system, so no extra browser engine download is needed:

   ```bash
   cd docs && npm install
   # To use Playwright Chromium, install it and change launch({ channel: "chrome" }) to launch()
   npx playwright install chromium
   ```

## Running

Run this in the **`docs/` directory**:

```bash
npm run screenshots
```

You can override the default address / credentials with environment variables:

```bash
SL_URL=http://localhost:58091 SL_USER=admin SL_PASS=admin npm run screenshots
```

The script prints the capture progress for each image, and the output overwrites `docs/public/screenshots/`. When it's done, run `cd docs && npm run docs:dev` locally to eyeball the landing page, and commit these PNGs only after confirming everything looks right.

## Which Images Are Captured

Each page is captured in both **light + dark** variants; the landing page components switch automatically based on the documentation site's current theme. Naming rule: light has no suffix, dark carries a `-dark` suffix (e.g., `home-desktop.png` / `home-desktop-dark.png`).

| File (`<page>-<viewport>[-dark].png`) | Description |
|------|------|
| `home-{desktop,mobile}` | Home page |
| `library-{desktop,mobile}` | Song library |
| `playlists-{desktop,mobile}` | Playlist list |
| `playlist-detail-{desktop,mobile}` | Playlist detail |
| `settings-{desktop,mobile}` | Settings |
| `player-mobile` | Immersive full-screen player (only mobile has the full-screen Now Playing) |

- Switch light/dark using `theme_mode` in the server `/settings/user-preferences` endpoint and synchronize local `flutter.theme_mode`. Restore the original server preference after capture.
- Desktop viewport `1440×900`, device pixel ratio `1.5` (about 2160px wide); mobile viewport `390×844`, device pixel ratio `2` (about 780px wide).
- The landing page components use CSS to wrap desktop images in a "browser window frame" (`BrowserFrame.vue`) and mobile images in a "phone shell" (`DeviceFrame.vue`), and automatically load the `-dark` variant based on `isDark`, so the script only needs to store the **clean raw screenshots**.

## How It Works (Why It's Not a Simple Screen Capture)

The frontend is **Flutter Web (canvas-rendered)**. There are no regular DOM form controls on the page, so Playwright's DOM selectors cannot reach the input fields and buttons. The script therefore uses:

1. **Coordinate + keyboard login**: After Flutter's first frame renders, it clicks the username / password fields according to the login page layout and types with the keyboard, then clicks the login button. The coordinates are consolidated in the `LOGIN` constant at the top of the script; if the login page is redesigned, they need corresponding fine-tuning.
2. **Reusing the login state**: After a successful login, `context.storageState()` saves the login info (the `flutter.access_token` etc. in `localStorage`) and injects it into new contexts for the multiple desktop / mobile viewports, **so there's no need to align login coordinates separately for each viewport**.
3. **Direct hash-route navigation**: Each page is navigated directly by routes like `#/library`, `#/playlists/1` for the screenshot (routes taken from `songloft-player-flutter`'s `AppRoutes`).
4. **Light / dark sets**: Update the server `theme_mode`, copy the login state with a matching `flutter.theme_mode`, and capture all pages again (the dark files carry the `-dark` suffix).
5. **Immersive player**: On mobile, it enters the playlist detail, taps the first song to start playing, then taps the bottom mini-player to expand to full screen and takes the screenshot.

## Where to Change Things When Adjustments Are Needed

The tunable items are consolidated at the top of the script:

- `LOGIN`: login page control coordinates (fine-tune when the login page is redesigned).
- `PAGES`: the pages to capture and their corresponding hash routes (add/remove pages).
- `VIEWPORTS` / `SCALE`: viewport sizes and device pixel ratios (resolution, sharpness).
- The click coordinates in the immersive player section: fine-tune when the play page layout is redesigned.

> Tip: Coordinates depend on the specific layout and are a fragile spot that requires "visual proofreading." Always manually go through the output after a run — if some image landed on a "page not found" or the login page, chances are the coordinates or routes need updating.

# Songloft FAQ

## Installation & Deployment

### Q: How should I download Songloft?

A:
- **Backend server**: Download the build for your system from [GitHub Releases](https://github.com/songloft-org/songloft/releases). Linux, macOS, and Windows are supported, with both binary and Docker image deployment options.
- **Flutter client**: Download the prebuilt installers from [Flutter client Releases](https://github.com/songloft-org/songloft-player/releases). Android, iOS, macOS, Windows, Linux, and Web are supported.

### Q: Which operating systems and architectures are supported?

A:

**Backend server**:
- **Linux**: x86_64, ARM64, ARMv7
- **macOS**: x86_64 (Intel), ARM64 (Apple Silicon)
- **Windows**: x86_64, ARM64

**Flutter client**:
- Android, iOS, macOS, Windows, Linux, Web (6 platforms)

### Q: What should I do if the container can't access music files during Docker deployment?

A: Make sure you mount volumes using absolute paths:
```bash
docker run -d \
  -v /absolute/path/to/music:/app/music \
  -v /absolute/path/to/data:/app/data \
  songloft/songloft:latest
```

### Q: What should I do if scheduled task times are wrong (incorrect timezone) during Docker deployment?

A: You need to set the `TZ` environment variable to specify the timezone:

```bash
docker run -d \
  -e TZ=Asia/Shanghai \
  -v /absolute/path/to/music:/app/music \
  -v /absolute/path/to/data:/app/data \
  songloft/songloft:latest
```

Add it in Docker Compose as well:
```yaml
environment:
  - TZ=Asia/Shanghai
```

### Q: How do I deploy under a sub path via a reverse proxy?

A: Songloft supports configuring a sub path via the `-base-path` argument or the `BASE_PATH` environment variable. This is useful when consolidating multiple services onto the same port behind an Nginx reverse proxy.

**Startup configuration**:
```bash
# Command-line argument
./songloft -base-path /songloft

# Or environment variable
BASE_PATH=/songloft ./songloft

# Docker
docker run -d -e BASE_PATH=/songloft ...
```

**Nginx configuration example**:
```nginx
location /songloft/ {
    proxy_pass http://127.0.0.1:58091;
    proxy_read_timeout 52w;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}
```

Once configured, you can access it at `http://your-domain/songloft/`. The Flutter embedded mode automatically detects the sub path from `<base href>`.

### Q: What's the difference between the full version and the lite version?

A:
- **Full version** (default): Embeds the Flutter Web frontend into the Go binary, so you can use the web interface directly by visiting the backend address.
- **Lite version** (`-tags lite`): Does not include the frontend and provides only the API service. You need to deploy the frontend separately or use a client.

## Configuration & Running

### Q: How do I configure multiple music directories?

A: Songloft only supports setting a single music root directory, but the scanner **fully supports symlinks** (symbolic links), so you can aggregate multiple directories by creating symlinks under the root directory.

**Linux / macOS**:

```bash
# Assume the music root is /app/music (Docker default) or ~/music
# Mount other directories into the root via symlinks

ln -s /mnt/nas/music /app/music/nas-music
ln -s /home/user/downloads/music /app/music/downloads
```

The scanner automatically follows symlinks and scans recursively, and detects circular links to prevent infinite loops.

**Docker deployment**: Simply mount multiple volumes to subdirectories of the music root — no symlinks needed:

```yaml
services:
  songloft:
    image: songloft/songloft:latest
    volumes:
      - /path/to/data:/app/data
      - /mnt/nas/music:/app/music/nas-music
      - /home/user/local-music:/app/music/local-music
```

**Windows**: Use `mklink /D` to create a directory link:

```cmd
mklink /D C:\music\nas-music \\NAS\music
```

> **Note**: Excluded directories (`exclude_dirs`) apply to all subdirectories, including those brought in via symlinks.

### Q: How do I change the server port?

A: There are two ways:
1. Use a command-line argument: `./songloft -port 8080`
2. Use an environment variable: `LISTEN_PORT=8080 ./songloft`

The default port is **58091**. Command-line arguments take precedence over environment variables.

### Q: How do I change the default password?

A: The default account and password are `admin` / `admin`, and changing them is recommended. Choose the method that matches your deployment.

> **Tip**: Docker users are recommended to set the password via the `ADMIN_PASSWORD` environment variable, and avoid the command-line argument approach (which can fail to take effect because an old process wasn't stopped).

**Docker startup**: Set it via the `ADMIN_PASSWORD` environment variable:
```bash
docker run -d \
  --name songloft \
  -p 58091:58091 \
  -v /path/to/music:/app/music \
  -v /path/to/data:/app/data \
  -e ADMIN_USERNAME=admin \
  -e ADMIN_PASSWORD=your_secure_password \
  songloft/songloft:latest
```

**Docker Compose startup**: Modify `ADMIN_PASSWORD` in `docker-compose.yml`:
```yaml
services:
  songloft:
    image: songloft/songloft:latest
    container_name: songloft
    restart: always
    ports:
      - "58091:58091"
    volumes:
      - /path/to/music:/app/music
      - /path/to/data:/app/data
    environment:
      - ADMIN_USERNAME=admin
      - ADMIN_PASSWORD=your_secure_password
      - LISTEN_PORT=58091
```

**Command-line startup**: Use the `-password` argument:
```bash
./songloft -password your_secure_password
```

**Windows**: Create a `songloft.bat` file in the same directory as `songloft.exe`, with the content:
```bat
songloft.exe -password your_secure_password
```
Then double-click `songloft.bat` to start (note: not `songloft.exe`).

### Q: Which music file formats are supported?

A: Mainstream audio formats are supported: **MP3, FLAC, WAV, APE, OGG, M4A, MOV, WMA, AIF/AIFF**, and more (MOV is a QuickTime container handled the same as the M4A family, common in some download sources). You can customize the list of supported formats via the `scan_config` database configuration.

### Q: How do I check the current version?

A:
- Command line: `./songloft -help` (prints version information)
- API: `curl http://localhost:58091/api/v1/version`

## Usage Questions

### Q: What should I do if a music file won't play?

A: Check the following:
1. Confirm the music file format is supported
2. Make sure the music file path is configured correctly
3. Check that file permissions allow reading
4. Optionally install `ffprobe` to obtain more complete audio technical parameters
5. For network songs, check that the URL is accessible

### Q: How do I scan the music library?

A: After adding or modifying music files, you **must manually trigger a scan** for them to appear in the song library. In the client, go to **Settings → Scan Management** and click the scan button (note: it's a long bar-shaped button, not a dialog). Scanning runs asynchronously; you can check the status via the progress endpoint, and you can also cancel an in-progress scan.

### Q: How does the Flutter client connect to the backend?

A:
- **Embedded mode** (Web): Flutter Web is embedded in the Go backend and automatically uses the same-origin address — no configuration needed.
- **Standalone deployment mode**: Enter the backend server address on the login page (e.g., `http://192.168.1.100:58091`); the address is saved automatically.

### Q: On Firefox, small controls in the Web client aren't clickable / clicks are offset. What should I do?

A: This is a **known click hit-test coordinate offset issue with Flutter Web (the CanvasKit rendering engine) on Firefox-family browsers** (see [flutter/flutter#182764](https://github.com/flutter/flutter/issues/182764), [flutter/flutter#117531](https://github.com/flutter/flutter/issues/117531)). It is rendering-engine/browser-layer behavior, not a Songloft resolution-adaptation problem. Typical symptoms: large settings items can be toggled but small controls can't be hit; JS plugin pages (native HTML) are unaffected.

**Fix and workarounds:**
- Prefer a **Chromium-based browser** (Chrome / Edge / Chromium) for the Web client to avoid the issue entirely.
- If you must use Firefox, try adjusting the page zoom level (`Ctrl` + `+` / `-`) until small controls become clickable — the offset varies with zoom, so at some levels the coordinates align.

### Q: How do I install and use plugins?

A:
1. In **Plugin Management** on the settings page, upload a plugin file in `.jsplugin.zip` format
2. After upload, the plugin automatically parses `plugin.json` and verifies permissions
3. Click the **Enable** button to activate the plugin (a subroute is registered after the two-layer SHA256 verification passes)
4. Enabled plugins show an entry on the home page

### Q: What should I do if I get a Token storage error on macOS?

A: Flutter's `secure_storage` may be unable to use the Keychain in an unsigned sandbox environment on macOS. Songloft has a built-in fallback mechanism that automatically falls back to `SharedPreferences` storage, without affecting normal usage.

### Q: How do I add network songs or internet radio?

A: In the **Radio Favorites** playlist in the client, click the add button and enter the radio streaming address (e.g., `.m3u`, `.pls`, or a direct audio stream URL). Network songs can be searched and added to playlists via JS plugins. Scanning local `.m3u` files to automatically import radio stations is not currently supported.

### Q: How do I operate on TV?

A: Songloft supports TV (screen width ≥ 1920px). Navigate with the remote's directional keys (D-pad); the focused element has a highlighted border and a scaling effect.

## Upgrades & Maintenance

### Q: How do I upgrade or update Songloft?

A:
- **Binary deployment**: Download the latest version, replace the old file, and restart the service
- **Docker deployment**: You can check for and perform upgrades online via **Upgrade Management** on the settings page
- **Flutter client**: The settings page notifies you of new frontend versions, with a link to download from GitHub Releases

The database migrates automatically — no manual action required. If something goes wrong after an upgrade, you can try deleting `data/songloft.db` and restarting (this will lose user data).

### Q: How do I verify the integrity of downloaded files?

A: Every Release includes a `checksums.txt` file:
```bash
wget https://github.com/songloft-org/songloft/releases/latest/download/checksums.txt
sha256sum -c checksums.txt
```

## API Usage

### Q: How do I obtain an access token via the API?

A:
```bash
curl -X POST http://localhost:58091/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"your_password"}'
```

The response contains an `access_token` and a `refresh_token`. The Access Token is used for everyday API access, and the Refresh Token is used to refresh an expired Access Token.

### Q: How do I use the Token in API requests?

A: Add an Authorization header to the request:
```bash
curl -X GET http://localhost:58091/api/v1/songs \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

Music files, covers, and lyrics are all accessed via song ID endpoints, authenticated with the `access_token` query parameter:
```
http://localhost:58091/api/v1/songs/{song_id}/play?access_token=YOUR_TOKEN
http://localhost:58091/api/v1/songs/{song_id}/cover?access_token=YOUR_TOKEN
http://localhost:58091/api/v1/songs/{song_id}/lyric?access_token=YOUR_TOKEN
```

> The legacy `/music/*` and `/cover/*` Base62-encoded paths have been retired.

### Q: How do I view the API documentation?

A: Start in development mode (`make run`) and visit `http://localhost:58091/swagger/index.html` to view the interactive Swagger documentation. Swagger is not included in production.

## System Requirements

### Q: Is ffprobe required?

A: **No, it's not required.** Songloft uses a pure Go library (`hanxi/tag`) to extract audio metadata and covers, with no external dependencies. Installing `ffprobe` lets you obtain more precise audio technical parameters (duration, bitrate, sample rate). The Docker image already includes ffprobe.

### Q: What do I need for a development environment?

A:
- **Backend development**: Go 1.26+
- **Frontend development**: Flutter 3.29+ / Dart 3.7+
- **Android builds**: You must first accept the SDK licenses with `sdkmanager --licenses`

---

## Getting Help

- **GitHub Issues**: [https://github.com/songloft-org/songloft/issues](https://github.com/songloft-org/songloft/issues)
- **Project homepage**: [https://github.com/songloft-org/songloft](https://github.com/songloft-org/songloft)
- **Flutter client**: [https://github.com/songloft-org/songloft-player](https://github.com/songloft-org/songloft-player)

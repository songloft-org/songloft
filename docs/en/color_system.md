# Songloft Color System Guide

This document explains the color system used in the Songloft project and the guidelines for using it.

## 📚 Table of Contents

- [Flutter Material 3 Color System](#flutter-material-3-color-system)
- [Theme Configuration](#theme-configuration)
- [Theme Pack System](#theme-pack-system)
- [Color Usage Guidelines](#color-usage-guidelines)
- [Responsive Theme Adaptation](#responsive-theme-adaptation)

---

## Flutter Material 3 Color System

The Songloft Flutter frontend uses the **Material 3** design system, generating a complete color scheme automatically via `ColorScheme.fromSeed`.

### Core Configuration

```dart
// clients/player-flutter/lib/core/theme/app_theme.dart
class AppTheme {
  static const Color _defaultSeedColor = Color(0xFF415F91); // M3 Blue baseline

  static ThemeData lightTheme({
    ScreenType screenType = ScreenType.mobile,
    ThemePack? themePack, // Optional theme pack override
  }) {
    return _buildTheme(Brightness.light, screenType, themePack);
  }

  static ThemeData darkTheme({
    ScreenType screenType = ScreenType.mobile,
    ThemePack? themePack,
  }) {
    return _buildTheme(Brightness.dark, screenType, themePack);
  }
}
```

When a `themePack` is provided, the seedColor is replaced by the one defined in the theme pack, generating an entirely different palette.

### Advantages

1. **Automatic palette generation**: Generates complete light/dark color schemes automatically from a seed color
2. **Semantic roles**: Semantic color roles such as `primary`, `secondary`, `tertiary`, `error`
3. **Guaranteed contrast**: Material 3 automatically ensures text-to-background contrast meets accessibility standards
4. **Consistency**: All components automatically use a unified color scheme

### ColorScheme Color Roles

| Role | Purpose | Example |
|------|------|------|
| `primary` | Primary actions, emphasis elements | Play button, selected navigation state |
| `onPrimary` | Text/icons on top of primary | Button text |
| `primaryContainer` | Primary-tinted container background | Selected card background |
| `secondary` | Secondary actions | Auxiliary buttons |
| `tertiary` | Third-level emphasis | Tags, badges |
| `error` | Error states | Delete button, error messages |
| `surface` | Page/card background | Scaffold background |
| `onSurface` | Text on top of surface | Primary text |
| `onSurfaceVariant` | Secondary text | Subtitles, descriptive text |
| `outline` | Borders | Input field borders, dividers |
| `outlineVariant` | De-emphasized borders | List dividers |

---

## Theme Configuration

### Theme Modes

Songloft supports three theme modes:

- **Light mode**: A bright interface style
- **Dark mode**: An eye-friendly dark interface
- **Follow system**: Automatically follows the operating system setting

Theme switching is implemented through the `ThemeSelector` component, with state managed by `themeModeProvider`.

### Built-in Colors and Lyric Backgrounds

- Settings → Appearance offers six built-in palettes: Blue, Teal, Forest, Rose, Amber, and Violet. Each supports light, dark, and system modes. The choice is stored on the current device, outside server preference synchronization.
- An active server theme pack takes precedence. Restoring the default resumes the palette saved on the device.
- Flutter mobile and desktop full players extract cover colors including warm browns and monochrome artwork, cache by the actual resource URL, and reload on server changes. Client decoding is limited to 100px width, including external covers. Actual vibrant colors take priority over the dominant color so large black areas cannot hide the cover's colored regions. Cover colors blend into surface at 28%→18% in light mode and 24%→18% in dark mode. The lower gradient prefers muted color, falling back to vibrant/dominant color. Missing, loading, or failed covers use the theme gradient; increased contrast uses a solid surface.
- Active lyrics, translations, romaji, and karaoke highlights share a foreground color. The theme accent targets 7:1 against the current background, preferring HSL lightness-only changes so its hue survives; backgrounds that may still be visible during the song-change animation are checked at 4.5:1, so worst-case artwork cannot neutralize the accent. Other lines target 4.5:1. Cover color separation is verified independently of text contrast. Palette tokens live in `core/theme/built_in_palette.dart`; background and lyric colors are calculated in `player_backdrop.dart`.
- Lynx uses the same six palette names within its Apple semantic color system. A bounded scrim covers blurred artwork at 70%→82% in light mode and 76%→82% in dark mode, with independent lyric foregrounds targeting 7:1 / 4.5:1. Loading, missing, or failed covers use the theme gradient; increased contrast or reduced transparency unmounts the cover and uses a solid background. Calculations live in `clients/player-lynx/src/shared/theme/player-colors.ts`; navigation SVGs and plugin WebView theme messages follow the current palette.

### Font Configuration

```dart
ThemeData(
  fontFamilyFallback: const ['NotoSansSC', 'NotoSansKR', 'sans-serif'],
  // ...
)
```

- Uses the system font by default
- Chinese falls back to **Noto Sans SC** (bundled with the app)
- Korean falls back to **Noto Sans KR** (bundled with the app)

### Component Theme Customization

```dart
ThemeData(
  useMaterial3: true,
  appBarTheme: const AppBarTheme(centerTitle: false, elevation: 0),
  cardTheme: CardThemeData(elevation: 0, shape: RoundedRectangleBorder(...)),
  inputDecorationTheme: InputDecorationTheme(border: OutlineInputBorder(...), filled: true),
  navigationBarTheme: const NavigationBarThemeData(height: 64, ...),
  // ...
)
```

---

## Theme Pack System

Songloft supports customizing the app’s color scheme and visual style via `.songloft-theme` theme packs. Theme mode (light/dark/system) and theme packs are independent — one theme pack defines both light and dark color schemes.

### SongloftThemeExtension

Custom parameters specific to theme packs are injected via the `ThemeExtension` mechanism:

```dart
class SongloftThemeExtension extends ThemeExtension<SongloftThemeExtension> {
  // Original fields
  final List<Color>? playerGradientColors; // Player gradient colors
  final double cardRadius;                 // Card border radius (default AppRadius.md)
  final double controlRadius;              // Control border radius (default AppRadius.md)
  final double navigationRadius;           // Navigation border radius (default AppRadius.md)

  // Liquid Glass tokens
  final Color glassFill;          // Glass fill (light default 0xB8FFFFFF)
  final Color glassFillStrong;    // Strong glass fill (light default 0xD9FFFFFF)
  final Color glassBorder;        // Glass border (light default 0x73FFFFFF)
  final Color glassHighlight;     // Glass highlight (light default 0x99FFFFFF)
  final Color glassGlow;          // Glass primary glow (= glassBase, light default #3BAEEF)
  final Color glassGlowFaint;     // Faint glow (glassBase @ 0.10)
  final Color glassSheen;         // Sheen (glassBase @ 0.18)
  final String navigationStyle;   // Navigation bar style: 'standard' | 'capsule' (default 'standard')
}
```

`glassGlow` / `glassGlowFaint` / `glassSheen` are all derived from **glassBase**: glassBase comes from `ThemePackColors.glassColor`; without a theme pack it falls back to `#3BAEEF` (light) / `#5BC0F5` (dark).

Usage in components:

```dart
final ext = Theme.of(context).extension<SongloftThemeExtension>();
if (ext?.playerGradientColors != null) {
  // Use the theme pack gradient
}
// Liquid Glass example
Container(
  decoration: BoxDecoration(
    color: ext?.glassFill,
    border: Border.all(color: ext?.glassBorder ?? Colors.transparent),
  ),
)
```

### How Theme Packs Affect the Color System

When a user activates a theme pack:

1. **seedColor is replaced**: A new full palette is generated from the theme pack’s `light.seedColor` / `dark.seedColor`
2. **surface/background can be overridden**: If the theme pack specifies `backgroundColor` or `surfaceColor`, they `copyWith` override the auto-generated values
3. **Default player gradient**: When cover colors are unavailable, the first and last `playerGradient` colors are blended into the current surface; cover colors take precedence when available
4. **Border radii**: `cardRadius`, `controlRadius`, `navigationRadius` are injected into component themes
5. **Liquid Glass tint**: `glassColor` serves as glassBase to derive `glassGlow` / `glassGlowFaint` / `glassSheen`; light and dark can specify different values
6. **Navigation bar style**: `navigationStyle` controls the navigation bar appearance — `'standard'` (default indicator) or `'capsule'` (capsule bar)

### Related Documents

- 📖 [Theme Pack Development Guide](/theme-pack-guide) — Detailed creation workflow and best practices
- 🎨 [Online Theme Repository](https://github.com/songloft-org/songloft-themes) — Community theme packs

---

## Color Usage Guidelines

### ✅ Recommended

#### 1. Obtain Colors Through Theme

```dart
// Get the ColorScheme
final colorScheme = Theme.of(context).colorScheme;

// Primary color
Container(color: colorScheme.primary)
Text('Title', style: TextStyle(color: colorScheme.onSurface))

// Secondary text
Text('Description', style: TextStyle(color: colorScheme.onSurfaceVariant))

// Error state
Icon(Icons.error, color: colorScheme.error)
```

#### 2. Use TextTheme

```dart
final textTheme = Theme.of(context).textTheme;

Text('Large title', style: textTheme.headlineMedium)
Text('Body text', style: textTheme.bodyLarge)
Text('Caption', style: textTheme.bodySmall)
```

#### 3. Use the Built-in Colors of Material Components

```dart
// FilledButton automatically uses the primary color
FilledButton(onPressed: () {}, child: Text('Primary action'))

// OutlinedButton automatically uses the outline color
OutlinedButton(onPressed: () {}, child: Text('Secondary action'))

// TextButton automatically uses the primary color
TextButton(onPressed: () {}, child: Text('Text action'))
```

### ❌ Avoid

```dart
// Do not hardcode color values
Container(color: Color(0xFF415F91))  // ❌

// Do not use Colors constants (they do not follow the theme)
Text('Text', style: TextStyle(color: Colors.grey))  // ❌

// Use Theme instead
Container(color: Theme.of(context).colorScheme.primary)  // ✅
Text('Text', style: TextStyle(color: Theme.of(context).colorScheme.onSurfaceVariant))  // ✅
```

---

## Responsive Theme Adaptation

The theme dynamically adjusts component sizes based on screen type (Mobile / Tablet / Desktop):

### SnackBar

| Screen Type | Style |
|---------|------|
| Mobile | Default floating style |
| Desktop | Fixed width 480px, centered |

### FilledButton / OutlinedButton / TextButton

| Screen Type | Minimum Size |
|---------|---------|
| Desktop | 88 × 44 |
| Mobile / Tablet | Flutter framework default (not customized) |

> In the actual code (`app_theme.dart`), all three of `filledButtonTheme` / `outlinedButtonTheme` / `textButtonTheme` are gated by `isDesktop`: the theme is only set for Desktop (Desktop → 88×44). For Mobile / Tablet they are `null`, falling back to the Flutter framework default sizes.

### Dialog Maximum Width

| Screen Type | Maximum Width |
|---------|---------|
| Mobile | 300px |
| Tablet | 400px |
| Desktop | 480px |

---

## Cover Color Extraction

Songloft uses the `palette_generator` library to extract dominant colors from song cover images, used for dynamic coloring of the player interface:

```dart
// clients/player-flutter/lib/core/utils/color_extraction.dart
// Extract dominant colors from the cover image, applied to scenarios such as the player background gradient
```

---

## Changelog

- **2026-09-20**: Liquid Glass theme system
  - `SongloftThemeExtension` adds 8 Liquid Glass token fields (`glassFill`, `glassFillStrong`, `glassBorder`, `glassHighlight`, `glassGlow`, `glassGlowFaint`, `glassSheen`, `navigationStyle`)
  - `ThemePackColors` adds `glassColor` field, driving Liquid Glass tint
  - `ThemePack` adds `navigationStyle` field (`'standard'` / `'capsule'`)
  - Font fallback adds NotoSansKR (Korean)
- **2026-07-31**: Added theme pack system
  - Support for `.songloft-theme` packs to customize colors, radii, and player gradient
  - Added `SongloftThemeExtension` custom theme extension
  - `AppTheme.lightTheme()` / `darkTheme()` accept an optional `ThemePack` parameter
  - Online theme catalog: browse and install from [songloft-themes](https://github.com/songloft-org/songloft-themes)
- **2026-04-14**: Migrated to the Flutter Material 3 color system
  - Main frontend migrated to Flutter, using `ColorScheme.fromSeed` for automatic palette generation
  - seedColor: M3 Blue baseline (`#415F91`)
  - Added responsive theme adaptation (Mobile / Tablet / Desktop)
  - Added cover color extraction feature

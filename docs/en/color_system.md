# Songloft Color System Guide

This document explains the color system used in the Songloft project and the guidelines for using it.

## 📚 Table of Contents

- [Flutter Material 3 Color System](#flutter-material-3-color-system)
- [Theme Configuration](#theme-configuration)
- [Color Usage Guidelines](#color-usage-guidelines)
- [Responsive Theme Adaptation](#responsive-theme-adaptation)

---

## Flutter Material 3 Color System

The Songloft Flutter frontend uses the **Material 3** design system, generating a complete color scheme automatically via `ColorScheme.fromSeed`.

### Core Configuration

```dart
// songloft-player/lib/core/theme/app_theme.dart
class AppTheme {
  static const Color _seedColor = Color(0xFF415F91); // M3 Blue baseline

  static ThemeData lightTheme({ScreenType screenType = ScreenType.mobile}) {
    return _buildTheme(Brightness.light, screenType);
  }

  static ThemeData darkTheme({ScreenType screenType = ScreenType.mobile}) {
    return _buildTheme(Brightness.dark, screenType);
  }
}
```

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

### Font Configuration

```dart
ThemeData(
  fontFamilyFallback: const ['NotoSansSC', 'sans-serif'],
  // ...
)
```

- Uses the system font by default
- Chinese falls back to **Noto Sans SC** (bundled with the app)

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

The theme dynamically adjusts component sizes based on screen type (Mobile / Tablet / Desktop / TV):

### SnackBar

| Screen Type | Style |
|---------|------|
| Mobile | Default floating style |
| Desktop | Fixed width 480px, centered |
| TV | Fixed width 600px, larger padding |

### FilledButton / OutlinedButton / TextButton

| Screen Type | Minimum Size |
|---------|---------|
| Desktop | 88 × 44 |
| TV | 120 × 56 |
| Mobile / Tablet | Flutter framework default (not customized) |

> In the actual code (`app_theme.dart`), all three of `filledButtonTheme` / `outlinedButtonTheme` / `textButtonTheme` are gated by `isDesktopOrTv`: the theme is only set for Desktop or TV (Desktop → 88×44, TV → 120×56). For Mobile / Tablet they are `null`, falling back to the Flutter framework default sizes.

### Dialog Maximum Width

| Screen Type | Maximum Width |
|---------|---------|
| Mobile | 300px |
| Tablet | 400px |
| Desktop | 480px |
| TV | 600px |

### TV-Specific Sizes

The TV platform uses dedicated constants defined in the `TvTheme` class:

| Property | Value | Description |
|------|-----|------|
| Title font | 24sp | `fontSizeTitle` |
| Body font | 20sp | `fontSizeBody` |
| Caption font | 16sp | `fontSizeCaption` |
| Focus border | 4px | `focusBorderWidth` |
| Focus scale | 1.05x | `focusScale` |
| Grid columns | 4 | `gridColumns` |
| Content padding | 48px | `contentPadding` |

---

## Cover Color Extraction

Songloft uses the `palette_generator` library to extract dominant colors from song cover images, used for dynamic coloring of the player interface:

```dart
// songloft-player/lib/core/utils/color_extraction.dart
// Extract dominant colors from the cover image, applied to scenarios such as the player background gradient
```

---

## Changelog

- **2026-04-14**: Migrated to the Flutter Material 3 color system
  - Main frontend migrated to Flutter, using `ColorScheme.fromSeed` for automatic palette generation
  - seedColor: M3 Blue baseline (`#415F91`)
  - Added responsive theme adaptation (Mobile / Tablet / Desktop / TV)
  - Added TV-specific theme constants (`TvTheme`)
  - Added cover color extraction feature

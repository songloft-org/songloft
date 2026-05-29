# Songloft 颜色系统规范

本文档说明 Songloft 项目中使用的颜色体系及其使用规范。

## 📚 目录

- [Flutter Material 3 颜色体系](#flutter-material-3-颜色体系)
- [主题配置](#主题配置)
- [颜色使用规范](#颜色使用规范)
- [响应式主题适配](#响应式主题适配)

---

## Flutter Material 3 颜色体系

Songloft Flutter 前端使用 **Material 3** 设计系统，通过 `ColorScheme.fromSeed` 自动生成完整的颜色方案。

### 核心配置

```dart
// songloft-player/lib/core/theme/app_theme.dart
class AppTheme {
  static const Color _seedColor = Color(0xFF6366F1); // indigo-500

  static ThemeData lightTheme({ScreenType screenType = ScreenType.mobile}) {
    return _buildTheme(Brightness.light, screenType);
  }

  static ThemeData darkTheme({ScreenType screenType = ScreenType.mobile}) {
    return _buildTheme(Brightness.dark, screenType);
  }
}
```

### 优势

1. **自动配色**：从 seed color 自动生成完整的亮色/暗色配色方案
2. **语义化角色**：`primary`、`secondary`、`tertiary`、`error` 等语义化颜色角色
3. **对比度保证**：Material 3 自动确保文本与背景的对比度符合无障碍标准
4. **一致性**：所有组件自动使用统一的配色方案

### ColorScheme 颜色角色

| 角色 | 用途 | 示例 |
|------|------|------|
| `primary` | 主要操作、强调元素 | 播放按钮、导航选中态 |
| `onPrimary` | primary 上的文本/图标 | 按钮文字 |
| `primaryContainer` | 主色容器背景 | 选中卡片背景 |
| `secondary` | 次要操作 | 辅助按钮 |
| `tertiary` | 第三级强调 | 标签、徽章 |
| `error` | 错误状态 | 删除按钮、错误提示 |
| `surface` | 页面/卡片背景 | Scaffold 背景 |
| `onSurface` | surface 上的文本 | 主要文本 |
| `onSurfaceVariant` | 次要文本 | 副标题、说明文字 |
| `outline` | 边框 | 输入框边框、分割线 |
| `outlineVariant` | 弱化边框 | 列表分割线 |

---

## 主题配置

### 主题模式

Songloft 支持三种主题模式：

- **亮色模式**：明亮的界面风格
- **暗色模式**：护眼的暗色界面
- **跟随系统**：自动跟随操作系统设置

主题切换通过 `ThemeSelector` 组件实现，状态由 `themeModeProvider` 管理。

### 字体配置

```dart
ThemeData(
  fontFamilyFallback: const ['NotoSansSC', 'sans-serif'],
  // ...
)
```

- 默认使用系统字体
- 中文回退到 **Noto Sans SC**（随应用打包）

### 组件主题定制

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

## 颜色使用规范

### ✅ 推荐使用

#### 1. 通过 Theme 获取颜色

```dart
// 获取 ColorScheme
final colorScheme = Theme.of(context).colorScheme;

// 主色
Container(color: colorScheme.primary)
Text('标题', style: TextStyle(color: colorScheme.onSurface))

// 次要文本
Text('描述', style: TextStyle(color: colorScheme.onSurfaceVariant))

// 错误状态
Icon(Icons.error, color: colorScheme.error)
```

#### 2. 使用 TextTheme

```dart
final textTheme = Theme.of(context).textTheme;

Text('大标题', style: textTheme.headlineMedium)
Text('正文', style: textTheme.bodyLarge)
Text('说明', style: textTheme.bodySmall)
```

#### 3. 使用 Material 组件的内置颜色

```dart
// FilledButton 自动使用 primary 色
FilledButton(onPressed: () {}, child: Text('主要操作'))

// OutlinedButton 自动使用 outline 色
OutlinedButton(onPressed: () {}, child: Text('次要操作'))

// TextButton 自动使用 primary 色
TextButton(onPressed: () {}, child: Text('文本操作'))
```

### ❌ 避免使用

```dart
// 不要硬编码颜色值
Container(color: Color(0xFF6366F1))  // ❌

// 不要使用 Colors 常量（不跟随主题）
Text('文本', style: TextStyle(color: Colors.grey))  // ❌

// 应该使用 Theme
Container(color: Theme.of(context).colorScheme.primary)  // ✅
Text('文本', style: TextStyle(color: Theme.of(context).colorScheme.onSurfaceVariant))  // ✅
```

---

## 响应式主题适配

主题根据屏幕类型（Mobile / Tablet / Desktop / TV）动态调整组件尺寸：

### SnackBar

| 屏幕类型 | 样式 |
|---------|------|
| Mobile | 默认浮动样式 |
| Desktop | 固定宽度 480px，居中 |
| TV | 固定宽度 600px，更大内边距 |

### FilledButton

| 屏幕类型 | 最小尺寸 |
|---------|---------|
| 非 TV（Mobile / Tablet / Desktop） | 88 × 44 |
| TV | 120 × 56 |

> 实际代码 (`app_theme.dart`) 只按 `isTv` 分两档；Mobile / Tablet / Desktop 共用 88×44。

### 对话框最大宽度

| 屏幕类型 | 最大宽度 |
|---------|---------|
| Mobile | 300px |
| Tablet | 400px |
| Desktop | 480px |
| TV | 600px |

### TV 端专用尺寸

TV 端使用 `TvTheme` 类定义的专用常量：

| 属性 | 值 | 说明 |
|------|-----|------|
| 标题字体 | 24sp | `fontSizeTitle` |
| 正文字体 | 20sp | `fontSizeBody` |
| 副标题字体 | 16sp | `fontSizeCaption` |
| 焦点边框 | 4px | `focusBorderWidth` |
| 焦点缩放 | 1.05x | `focusScale` |
| 网格列数 | 4 | `gridColumns` |
| 内容内边距 | 48px | `contentPadding` |

---

## 封面颜色提取

Songloft 使用 `palette_generator` 库从歌曲封面图片中提取主色调，用于播放器界面的动态配色：

```dart
// songloft-player/lib/core/utils/color_extraction.dart
// 从封面图片提取主色调，应用到播放器背景渐变等场景
```

---

## 更新日志

- **2026-04-14**: 更新为 Flutter Material 3 颜色体系
  - 主前端迁移到 Flutter，使用 `ColorScheme.fromSeed` 自动配色
  - seedColor: indigo-500 (`#6366F1`)
  - 新增响应式主题适配（Mobile / Tablet / Desktop / TV）
  - 新增 TV 端专用主题常量（`TvTheme`）
  - 新增封面颜色提取功能

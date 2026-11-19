// 用 Playwright 抓取 Songloft Flutter Web 界面截图，供文档站落地页使用。
//
// 前置：本地运行 songloft 服务（默认 http://localhost:58091，账号 admin/admin），
//       并在 docs/ 下安装依赖（`npm install`，含 playwright；本脚本用系统 Chrome，无需下载浏览器）。
// 用法：在 docs/ 目录执行 `npm run screenshots`（等价于 node scripts/capture-screenshots.mjs）。
//       环境变量 SL_URL / SL_USER / SL_PASS 可覆盖地址与账号。
//
// 原理：前端是 Flutter Web（canvas 渲染，DOM 选择器抓不到控件），故：
//   1. 先在一个上下文里用「坐标 + 键盘」完成 UI 登录；
//   2. 用 ctx.storageState() 把登录态（localStorage 里的 flutter.access_token 等）
//      复用到桌面 / 移动多个视口，避免为每种视口分别对齐登录坐标；
//   3. 按 hash 路由（取自 songloft-player-flutter 的 AppRoutes）直达各页面截图；
//   4. 移动端额外点歌 + 展开迷你播放器，抓「沉浸式全屏播放器」；
//   5. 复制一份把 flutter.theme_mode 改为 dark 的登录态，再抓一套暗色图（-dark 后缀）。
import { chromium } from 'playwright'
import { mkdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

const OUT = resolve(dirname(fileURLToPath(import.meta.url)), '../public/screenshots')
mkdirSync(OUT, { recursive: true })

const BASE = process.env.SL_URL ?? 'http://localhost:58091'
const USER = process.env.SL_USER ?? 'admin'
const PASS = process.env.SL_PASS ?? 'admin'

// 登录页控件坐标（相对 1440x900 桌面视口，肉眼校对自登录页截图）
const LOGIN = { user: [720, 468], pass: [720, 532], submit: [720, 616] }

// 主题模式存储在服务端 user-preferences，登录后前端会以服务端值为准（覆盖 localStorage），
// 所以切主题必须走服务端 API，否则亮/暗两遍都会渲染成服务端当前的主题。
async function apiToken() {
  const r = await fetch(`${BASE}/api/v1/auth/login`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ username: USER, password: PASS }),
  })
  return (await r.json()).access_token
}
async function getServerTheme() {
  const token = await apiToken()
  const r = await fetch(`${BASE}/api/v1/settings/user-preferences`, {
    headers: { authorization: `Bearer ${token}` },
  })
  return (await r.json()).theme_mode
}
async function setServerTheme(mode) {
  const token = await apiToken()
  const cur = await (
    await fetch(`${BASE}/api/v1/settings/user-preferences`, { headers: { authorization: `Bearer ${token}` } })
  ).json()
  await fetch(`${BASE}/api/v1/settings/user-preferences`, {
    method: 'PUT',
    headers: { 'content-type': 'application/json', authorization: `Bearer ${token}` },
    body: JSON.stringify({ ...cur, theme_mode: mode }),
  })
}

// 标准页面：name 输出文件名，route 为 hash 路由（空串=首页）。
const PAGES = [
  { name: 'home', route: '' },
  { name: 'library', route: '#/library' },
  { name: 'playlists', route: '#/playlists' },
  { name: 'playlist-detail', route: '#/playlists/1' },
  { name: 'settings', route: '#/settings' },
]

const VIEWPORTS = {
  desktop: { width: 1440, height: 900 },
  mobile: { width: 390, height: 844 },
}

// 每种视口的设备像素比：桌面 1.5（约 2160px 宽，清晰又不至过大），移动 2（约 780px 宽）。
const SCALE = { desktop: 1.5, mobile: 2 }

const waitFlutter = (page, ms = 2600) => page.waitForTimeout(ms)

async function uiLogin(browser) {
  const ctx = await browser.newContext({ viewport: VIEWPORTS.desktop })
  const page = await ctx.newPage()
  await page.goto(BASE)
  await waitFlutter(page, 3500)
  await page.mouse.click(...LOGIN.user); await page.waitForTimeout(250)
  await page.keyboard.type(USER, { delay: 25 })
  await page.mouse.click(...LOGIN.pass); await page.waitForTimeout(250)
  await page.keyboard.type(PASS, { delay: 25 })
  await page.mouse.click(...LOGIN.submit)
  await waitFlutter(page, 4500)
  if (page.url().includes('/login')) throw new Error('登录失败：仍停留在登录页，请检查坐标/账号')
  const state = await ctx.storageState()
  await ctx.close()
  return state
}

// 复制一份登录态，把 flutter.theme_mode 覆盖为指定主题（light/dark/system）
function withTheme(state, mode) {
  const clone = JSON.parse(JSON.stringify(state))
  for (const origin of clone.origins ?? []) {
    const item = (origin.localStorage ?? []).find((e) => e.name === 'flutter.theme_mode')
    if (item) item.value = JSON.stringify(mode)
  }
  return clone
}

async function newCtx(browser, vp, storageState) {
  const ctx = await browser.newContext({ viewport: VIEWPORTS[vp], deviceScaleFactor: SCALE[vp], storageState })
  return { ctx, page: await ctx.newPage() }
}

async function shot(page, file) {
  await page.screenshot({ path: `${OUT}/${file}.png` })
  console.log('  ✓', `${file}.png`)
}

const browser = await chromium.launch({ channel: 'chrome' })
const originalTheme = await getServerTheme() // 结束后恢复用户原本的主题偏好
try {
  console.log('登录中…')
  const state = await uiLogin(browser)
  console.log('登录成功，开始抓图')

  // 亮色 / 暗色各抓一套：亮色文件名无后缀，暗色文件名带 -dark 后缀。
  // 每遍先用服务端 API 切主题（权威来源），localStorage 也同步为同一值。
  const themes = [
    { suffix: '', mode: 'light' },
    { suffix: '-dark', mode: 'dark' },
  ]

  for (const { suffix, mode } of themes) {
    console.log(suffix ? '· 暗色' : '· 亮色')
    await setServerTheme(mode)
    const st = withTheme(state, mode)

    // 标准页面：桌面 + 移动
    for (const vp of Object.keys(VIEWPORTS)) {
      const { ctx, page } = await newCtx(browser, vp, st)
      for (const p of PAGES) {
        await page.goto(`${BASE}/${p.route}`)
        await waitFlutter(page)
        await shot(page, `${p.name}-${vp}${suffix}`)
      }
      await ctx.close()
    }

    // 沉浸式全屏播放器（仅移动端有全屏 Now Playing）
    {
      const { ctx, page } = await newCtx(browser, 'mobile', st)
      await page.goto(`${BASE}/#/playlists/1`); await waitFlutter(page, 3000)
      await page.mouse.click(195, 365); await waitFlutter(page, 2800) // 点第一首开始播放
      await page.mouse.click(195, 772); await waitFlutter(page, 2800) // 点迷你播放器展开全屏
      await shot(page, `player-mobile${suffix}`)
      await ctx.close()
    }
  }

  console.log('完成 →', OUT)
} finally {
  await setServerTheme(originalTheme).catch(() => {}) // 恢复原主题偏好
  await browser.close()
}

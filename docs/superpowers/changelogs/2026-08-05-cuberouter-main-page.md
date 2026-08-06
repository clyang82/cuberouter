# Changelog

All notable changes to this project are documented here, newest first.

## Unreleased

### Changed

#### CubeRouter Main Page (v1)

New API is rebranded as CubeRouter, and the landing page is replaced with the CubeRouter design, alongside i18n and theme updates.

**Branding**

- System name, page titles, startup logs, Electron window/tray labels and error messages renamed from "New API" to "CubeRouter".
- Favicon/logo assets replaced: `favicon.ico` dropped, `head.png` added, `logo.png` and `logo.tsx` updated.

**Landing page**

- Old home page sections (Hero, Stats, Features, HowItWorks, CTA, terminal demo, gateway cards, scrolling icons, icon mapper) removed and replaced with a single new `Landing` component (`landing/landing.tsx` + `landing.css`) ported from CubeRouter, including an interactive WebGL globe (`landing/globe.tsx`) powered by the new `cobe` dependency.
- Home now renders just `<Landing />` inside `PublicLayout`; the default footer is no longer rendered on the landing page.

**i18n**

- fr, ru, ja, and vi locales dropped entirely (locale files, language options, i18n resources, and untranslated reports); supported languages are now en, zhCN, and zhTW.
- `sync-i18n.mjs` untranslated detection now only flags zh/zh-TW strings identical to English.

**Theme & typography**

- UI sans font switched from Public Sans to Bricolage Grotesque (with PingFang SC / Microsoft YaHei CJK fallbacks); JetBrains Mono added.
- CubeRouter brand accent (#FB6415 orange with black ink) applied as the primary/sidebar-primary color across every theme preset and color scheme.

**Known issues**

- The `sync-i18n.mjs` comment references `zh-TW`, but the `locale === 'zh'` check only matches the literal `zh` code — if the script's locale codes are `zhCN`/`zhTW` rather than `zh`, that untranslated-detection branch may never fire.

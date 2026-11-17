import { defineConfig, type DefaultTheme } from 'vitepress'
import type MarkdownIt from 'markdown-it'

// The site is served from https://<user>.github.io/sugar-pet/.  The workflow
// passes the real base path of the Pages site (DOCS_BASE), so a custom domain
// needs no change here.
const base = (process.env.DOCS_BASE || '/sugar-pet').replace(/\/+$/, '') + '/'

const repo = 'https://github.com/lijiaxiang63/sugar-pet'

// One sidebar for both languages: `prefix` is '' or '/zh', `t` picks the text.
function sidebar(prefix: string, t: (en: string, zh: string) => string): DefaultTheme.SidebarItem[] {
  const link = (path: string) => prefix + path
  return [
    {
      text: t('Getting started', '开始之前'),
      items: [
        { text: t('What SUGAR-PET does', 'SUGAR-PET 是什么'), link: link('/guide/introduction') },
        { text: t('Install the app', '安装应用'), link: link('/guide/install') },
        { text: t('A tour of the window', '认识界面'), link: link('/guide/interface') },
        { text: t('Prepare your data', '准备数据'), link: link('/guide/prepare-data') },
      ],
    },
    {
      text: t('Step-by-step tutorial', '分步教程'),
      items: [
        { text: t('Overview', '教程总览'), link: link('/tutorial/') },
        { text: t('1. Warp to MNI space', '1. 空间标准化'), link: link('/tutorial/spatial') },
        { text: t('2. Build the reference', '2. 建立参考库'), link: link('/tutorial/build-reference') },
        { text: t('3. Normalise the intensity', '3. 强度归一化'), link: link('/tutorial/normalize') },
        { text: t('4. Z-score and W-score maps', '4. Z 值图与 W 值图'), link: link('/tutorial/zmap') },
        { text: t('5. Classify parkinsonism', '5. 帕金森综合征分类'), link: link('/tutorial/classify') },
        { text: t('6. Compare two groups', '6. 组间比较'), link: link('/tutorial/compare') },
        { text: t('7. Reference cluster', '7. 参考簇'), link: link('/tutorial/reference-cluster') },
        { text: t('Masks and atlas', '掩膜与图谱'), link: link('/tutorial/masks') },
      ],
    },
    {
      text: t('Reference', '参考'),
      items: [
        { text: t('Choosing a method', '方法怎么选'), link: link('/reference/methods') },
        { text: t('Output files', '输出文件一览'), link: link('/reference/outputs') },
        { text: t('Command line', '命令行对照'), link: link('/reference/cli') },
        { text: t('Questions and problems', '常见问题'), link: link('/reference/faq') },
      ],
    },
  ]
}

function nav(prefix: string, t: (en: string, zh: string) => string): DefaultTheme.NavItem[] {
  return [
    { text: t('Guide', '指南'), link: prefix + '/guide/introduction', activeMatch: '/guide/' },
    { text: t('Tutorial', '教程'), link: prefix + '/tutorial/', activeMatch: '/tutorial/' },
    { text: t('Reference', '参考'), link: prefix + '/reference/methods', activeMatch: '/reference/' },
    { text: t('Download', '下载'), link: repo + '/releases/latest' },
  ]
}

// A line break inside a paragraph is shown as a space, which is wrong between
// two Chinese characters or next to Chinese punctuation: drop it there.
const han = /[\u2E80-\u2FFF\u3040-\u30FF\u3400-\u9FFF\uF900-\uFAFF]/
const hanPunctuation = /[\u3000-\u303F\uFF00-\uFFEF\u2014\u2018\u2019\u201C\u201D\u2026]/
function joinChineseLines(md: MarkdownIt) {
  md.renderer.rules.softbreak = (tokens, idx) => {
    let before = ''
    for (let i = idx - 1; i >= 0 && !before; i--) before = tokens[i].content.slice(-1)
    let after = ''
    for (let i = idx + 1; i < tokens.length && !after; i++) after = tokens[i].content.slice(0, 1)
    const drop = hanPunctuation.test(before) || hanPunctuation.test(after) || (han.test(before) && han.test(after))
    return drop ? '' : '\n'
  }
}

// The numbered list under a <Shot> gets the look of the screenshot's marks
// (class "steps" of the theme), and so does a list that goes on counting from
// an earlier one ("7. ..."): its numbers are marks of a screenshot too.
function stepLists(md: MarkdownIt) {
  md.core.ruler.push('step_lists', (state) => {
    state.tokens.forEach((token, i) => {
      if (token.type !== 'ordered_list_open' || token.level !== 0) return
      const start = Number(token.attrGet('start') ?? 1)
      // A component on a line of its own is an HTML token, then an empty inline one.
      let j = i - 1
      while (j >= 0 && state.tokens[j].type === 'inline' && !state.tokens[j].content) j--
      const before = state.tokens[j]
      const underShot = !!before && before.type.startsWith('html_') && before.content.trimStart().startsWith('<Shot')
      if (!underShot && start === 1) return
      token.attrJoin('class', 'steps')
      if (start > 1) token.attrSet('style', `--start: ${start - 1}`)
    })
  })
}

const en = (text: string, _zh: string) => text
const zh = (_en: string, text: string) => text

export default defineConfig({
  base,
  title: 'SUGAR-PET',
  cleanUrls: true,
  lastUpdated: true,
  // The flowcharts of the classification live here too, for the READMEs.
  srcExclude: ['scripts/**'],
  markdown: {
    config(md) {
      joinChineseLines(md)
      stepLists(md)
    },
  },
  head: [['link', { rel: 'icon', type: 'image/png', href: base + 'favicon.png' }]],

  themeConfig: {
    logo: '/logo.png',
    socialLinks: [{ icon: 'github', link: repo }],
    search: {
      provider: 'local',
      options: {
        locales: {
          zh: {
            translations: {
              button: { buttonText: '搜索文档', buttonAriaLabel: '搜索文档' },
              modal: {
                noResultsText: '没有找到相关结果',
                resetButtonTitle: '清除查询条件',
                displayDetails: '显示详细列表',
                backButtonTitle: '关闭搜索',
                footer: { selectText: '选择', navigateText: '切换', closeText: '关闭' },
              },
            },
          },
        },
      },
    },
  },

  locales: {
    root: {
      label: 'English',
      lang: 'en-US',
      description: 'User guide of SUGAR-PET, the desktop app for intensity normalisation of brain FDG-PET images.',
      themeConfig: {
        nav: nav('', en),
        sidebar: sidebar('', en),
        outline: { level: [2, 3] },
        footer: {
          message: 'For research use only; not a medical device.',
          copyright: 'Released under the MIT License.',
        },
      },
    },
    zh: {
      label: '简体中文',
      lang: 'zh-CN',
      link: '/zh/',
      description: 'SUGAR-PET 使用文档：脑 FDG-PET 强度归一化桌面应用。',
      themeConfig: {
        nav: nav('/zh', zh),
        sidebar: sidebar('/zh', zh),
        outline: { level: [2, 3], label: '本页目录' },
        docFooter: { prev: '上一页', next: '下一页' },
        lastUpdated: { text: '最后更新于' },
        returnToTopLabel: '回到顶部',
        sidebarMenuLabel: '菜单',
        darkModeSwitchLabel: '外观',
        lightModeSwitchTitle: '切换到浅色模式',
        darkModeSwitchTitle: '切换到深色模式',
        langMenuLabel: '切换语言',
        footer: {
          message: '仅供科研使用，不是医疗器械。',
          copyright: '代码采用 MIT 许可。',
        },
      },
    },
  },
})

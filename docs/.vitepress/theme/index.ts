import type { Theme } from 'vitepress'
import DefaultTheme from 'vitepress/theme'
import Shot from './Shot.vue'
import './custom.css'

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    // <Shot name="reference" :marks="[[1, 120]]" /> in any page.
    app.component('Shot', Shot)
  },
} satisfies Theme

<script setup lang="ts">
// A screenshot of the app in the language of the page, with numbered marks
// that match the numbered steps under it.
//
//   <Shot name="reference" alt="The Reference page" :marks="[[1, 127], [2, 380]]" />
//
// `name` is a file of public/screenshots/<en|zh>/ (see scripts/screenshots.sh).
// A mark is [number, y] or [number, x, y], in points of the 1060 x 900 window
// the screenshots are drawn at.  Without x the mark sits in the margin left
// of the page's cards; 'run' as y puts it on the button that starts the task.
// Text wraps differently in the two languages, so each page gives the marks
// of its own language.
//
// A screenshot named "settings-<section>" is of the settings window, which
// is 820 x 620: its marks are in the points of that window, and it is shown
// narrower, at the scale of the other screenshots.
import { computed } from 'vue'
import { useData, withBase } from 'vitepress'

type Mark = [number, number | 'run'] | [number, number, number]

const props = defineProps<{ name: string; alt?: string; marks?: Mark[] }>()

const MAIN = { width: 1060, height: 900, margin: 257 } // margin: between the edge of the page and its cards
const SETTINGS = { width: 820, height: 620, margin: 228 }
const size = computed(() => (props.name.startsWith('settings-') ? SETTINGS : MAIN))

const { lang } = useData()
const src = computed(() => withBase(`/screenshots/${lang.value.startsWith('zh') ? 'zh' : 'en'}/${props.name}.png`))

const marks = computed(() =>
  (props.marks ?? []).map((mark) => {
    const { width, height, margin } = size.value
    let [n, x, y] = mark.length === 3 ? mark : [mark[0], margin, mark[1]]
    if (y === 'run') [x, y] = [1002, 845]
    return { n, left: `${(x / width) * 100}%`, top: `${((y as number) / height) * 100}%` }
  }),
)
</script>

<template>
  <figure class="shot" :class="{ narrow: size === SETTINGS }">
    <a :href="src" target="_blank" rel="noopener" class="shot-frame">
      <img :src="src" :alt="alt ?? ''" :width="size.width" :height="size.height" loading="lazy" />
      <span v-for="mark in marks" :key="mark.n" class="shot-mark" :style="{ left: mark.left, top: mark.top }">{{ mark.n }}</span>
    </a>
    <figcaption v-if="alt">{{ alt }}</figcaption>
  </figure>
</template>

<style scoped>
.shot {
  margin: 20px 0 24px;
}

/* The settings window is smaller than the main one: keep its scale. */
.shot.narrow .shot-frame {
  max-width: max(78%, 420px);
  margin: 0 auto;
}

.shot-frame {
  position: relative;
  display: block;
  container-type: inline-size;
  border: 1px solid var(--vp-c-divider);
  border-radius: 10px;
  overflow: hidden;
  box-shadow: 0 6px 24px rgba(0, 0, 0, 0.08);
  /* The screenshots are of the light appearance. */
  background: #f0eee6;
}

.shot-frame img {
  display: block;
  width: 100%;
  height: auto;
}

.shot-mark {
  position: absolute;
  transform: translate(-50%, -50%);
  display: grid;
  place-items: center;
  width: max(14px, var(--mark, 2.5cqw));
  height: max(14px, var(--mark, 2.5cqw));
  border-radius: 50%;
  background: #e5484d;
  color: #fff;
  font-size: max(9px, var(--mark-text, 1.45cqw));
  font-weight: 700;
  line-height: 1;
  box-shadow: 0 0 0 2px #fff, 0 2px 6px rgba(0, 0, 0, 0.3);
  pointer-events: none;
}

/* Marks of the same size as on the screenshots of the main window. */
.shot.narrow .shot-mark {
  --mark: 3.23cqw;
  --mark-text: 1.87cqw;
}

figcaption {
  margin-top: 8px;
  text-align: center;
  font-size: 13px;
  color: var(--vp-c-text-2);
}
</style>

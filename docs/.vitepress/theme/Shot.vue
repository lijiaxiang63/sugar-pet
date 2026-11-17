<script setup lang="ts">
// A screenshot of the app in the language of the page, with numbered marks
// that match the numbered steps under it.
//
//   <Shot name="reference" alt="The Reference page" :marks="[[1, 127], [2, 380]]" />
//
// `name` is a file of public/screenshots/<en|zh>/ (see scripts/screenshots.sh).
// A mark is [number, y] or [number, x, y], in points of the 1060 x 740 window
// the screenshots are drawn at.  Without x the mark sits in the margin left
// of the page's cards, which is the same place in both languages; 'run' as y
// puts it on the button that starts the task.
import { computed } from 'vue'
import { useData, withBase } from 'vitepress'

type Mark = [number, number | 'run'] | [number, number, number]

const props = defineProps<{ name: string; alt?: string; marks?: Mark[] }>()

const WIDTH = 1060
const HEIGHT = 740
const MARGIN = 262 // between the sidebar and the cards

const { lang } = useData()
const src = computed(() => withBase(`/screenshots/${lang.value.startsWith('zh') ? 'zh' : 'en'}/${props.name}.png`))

const marks = computed(() =>
  (props.marks ?? []).map((mark) => {
    let [n, x, y] = mark.length === 3 ? mark : [mark[0], MARGIN, mark[1]]
    if (y === 'run') [x, y] = [1037, 691]
    return { n, left: `${(x / WIDTH) * 100}%`, top: `${((y as number) / HEIGHT) * 100}%` }
  }),
)
</script>

<template>
  <figure class="shot">
    <a :href="src" target="_blank" rel="noopener" class="shot-frame">
      <img :src="src" :alt="alt ?? ''" :width="WIDTH" :height="HEIGHT" loading="lazy" />
      <span v-for="mark in marks" :key="mark.n" class="shot-mark" :style="{ left: mark.left, top: mark.top }">{{ mark.n }}</span>
    </a>
    <figcaption v-if="alt">{{ alt }}</figcaption>
  </figure>
</template>

<style scoped>
.shot {
  margin: 20px 0 24px;
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
  background: #f5f5f5;
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
  width: max(14px, 2.5cqw);
  height: max(14px, 2.5cqw);
  border-radius: 50%;
  background: #e5484d;
  color: #fff;
  font-size: max(9px, 1.45cqw);
  font-weight: 700;
  line-height: 1;
  box-shadow: 0 0 0 2px #fff, 0 2px 6px rgba(0, 0, 0, 0.3);
  pointer-events: none;
}

figcaption {
  margin-top: 8px;
  text-align: center;
  font-size: 13px;
  color: var(--vp-c-text-2);
}
</style>

#!/bin/sh
# Redraws the screenshots of the documentation, in English and Chinese, with
# the app's own snapshot mode (`sugar-pet --snapshot`, see app/snapshot.go):
#
#     sh docs/scripts/screenshots.sh
#
# Needs Go (to build the app) and pngquant (to keep the files small).  Run it
# after the look of a page changes, then check the numbered marks of the
# <Shot> pictures in the tutorial: they are placed by hand, for each language.
#
# With ImageMagick (`magick`) it also redraws the pictures of the READMEs,
# app/screenshots/<page>-<en|zh>.png.
set -eu

cd "$(dirname "$0")/../.."
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

(cd app && go build -o "$tmp/sugar-pet" .)

# The window of the screenshots is taller than the one the app opens, so that
# a screenshot holds everything its numbered steps name.
height=900

# <snapshot of the app>:<name in the docs>.  A snapshot "page-2" is that page
# scrolled down by one step, "page-end" its end, scrolled all the way down,
# and "settings-<section>" a section of the settings window, which is drawn
# at its own size (820 x 620, see Shot.vue).
shots="
welcome:welcome                 welcome-new:new-project
data:data                       data-end:data-covariates
spatial:spatial                 spatial-end:spatial-results
normalize:normalize             normalize-2:normalize-controls
normalize-3:normalize-output    normalize-end:normalize-results
zmap:zmap                       zmap-end:zmap-results
classify:classify               classify-2:classify-results
classify-end:classify-figure
quick:quick                     quick-2:quick-results
quick-end:quick-warp
compare:compare                 compare-end:compare-results
cluster:cluster                 cluster-end:cluster-results
masks:masks                     masks-end:masks-atlas
normalize-dark:normalize-dark
settings-general:settings-general
settings-spatial:settings-spatial
settings-normalize:settings-normalize
settings-reference:settings-controls
settings-classify:settings-classify
settings-compare:settings-compare
settings-cluster:settings-cluster
"

# The pictures of the READMEs, <snapshot>:<name>: the window as the app first
# opens it (1060 x 740), at half the size of the snapshot.
readme="normalize:normalize classify-2:classify compare:compare"

for lang in en zh; do
  "$tmp/sugar-pet" --snapshot "$tmp/$lang" "$lang" "$height"
  out=docs/public/screenshots/$lang
  rm -rf "$out"
  mkdir -p "$out"
  for shot in $shots; do
    pngquant --force --strip --quality 80-98 --output "$out/${shot#*:}.png" "$tmp/$lang/${shot%%:*}.png"
  done
done

du -sh docs/public/screenshots

if command -v magick >/dev/null; then
  for lang in en zh; do
    "$tmp/sugar-pet" --snapshot "$tmp/readme-$lang" "$lang" 740
    for shot in $readme; do
      magick "$tmp/readme-$lang/${shot%%:*}.png" -resize 50% "$tmp/readme.png"
      pngquant --force --strip --quality 80-98 --output "app/screenshots/${shot#*:}-$lang.png" "$tmp/readme.png"
    done
  done
  du -sh app/screenshots
else
  echo "ImageMagick (magick) not found: the pictures of the READMEs in app/screenshots are left as they are." >&2
fi

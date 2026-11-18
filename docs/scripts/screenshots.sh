#!/bin/sh
# Redraws the screenshots of the documentation, in English and Chinese, with
# the app's own snapshot mode (`sugar-pet --snapshot`, see app/snapshot.go):
#
#     sh docs/scripts/screenshots.sh
#
# Needs Go (to build the app) and pngquant (to keep the files small).  Run it
# after the look of a page changes, then check the numbered marks of the
# <Shot> pictures in the tutorial: they are placed by hand, for each language.
set -eu

cd "$(dirname "$0")/../.."
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

(cd app && go build -o "$tmp/sugar-pet" .)

# The window of the screenshots is taller than the one the app opens, so that
# a screenshot holds everything its numbered steps name.
height=900

# <snapshot of the app>:<name in the docs>.  A snapshot "page-end" is the end
# of that page, scrolled all the way down.
shots="
spatial:spatial                 spatial-end:spatial-results
reference:reference             reference-end:reference-settings
normalize:normalize             normalize-end:normalize-results
zmap:zmap                       zmap-end:zmap-results
classify:classify               classify-end:classify-results
compare:compare                 compare-end:compare-results
cluster:cluster                 cluster-end:cluster-results
masks:masks                     masks-end:masks-atlas
normalize-dark:normalize-dark
"

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

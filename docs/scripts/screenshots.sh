#!/bin/sh
# Redraws the screenshots of the documentation, in English and Chinese, with
# the app's own snapshot mode (`sugar-pet --snapshot`, see app/snapshot.go):
#
#     sh docs/scripts/screenshots.sh
#
# Needs Go (to build the app) and pngquant (to keep the files small).  Run it
# after the look of a page changes, then check the numbered marks of the
# <Shot> pictures in the tutorial: they are placed by hand.
set -eu

cd "$(dirname "$0")/../.."
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

(cd app && go build -o "$tmp/sugar-pet" .)

# <snapshot of the app>:<name in the docs>.  A snapshot "page-N" is the Nth
# screen of that page, scrolling down.
shots="
spatial:spatial                 spatial-2:spatial-results
reference:reference             reference-2:reference-settings
normalize:normalize             normalize-3:normalize-results
zmap:zmap                       zmap-3:zmap-results
classify:classify               classify-2:classify-results
compare:compare                 compare-3:compare-results
cluster:cluster                 cluster-2:cluster-results
masks:masks                     masks-2:masks-atlas
normalize-dark:normalize-dark
"

for lang in en zh; do
  if [ "$lang" = zh ]; then
    "$tmp/sugar-pet" --snapshot "$tmp/$lang" zh
  else
    "$tmp/sugar-pet" --snapshot "$tmp/$lang"
  fi
  out=docs/public/screenshots/$lang
  rm -rf "$out"
  mkdir -p "$out"
  for shot in $shots; do
    pngquant --force --strip --quality 80-98 --output "$out/${shot#*:}.png" "$tmp/$lang/${shot%%:*}.png"
  done
done

du -sh docs/public/screenshots

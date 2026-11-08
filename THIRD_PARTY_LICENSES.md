# Third-Party Licenses

Highlights Manager ships these third-party files in `static/`. They are committed
as-is (byte-identical to upstream) and copied into the Docker image with the rest
of `static/`, so the web UI loads nothing from external CDNs. The full license
text sits next to each file and is included in the image.

## Vendored JavaScript

### htmx 2.0.4

- **Author:** Big Sky Software
- **License:** 0BSD (Zero-Clause BSD)
- **Source:** https://github.com/bigskysoftware/htmx (npm `htmx.org@2.0.4`, `dist/htmx.min.js`)
- **Files:** `static/vendor/htmx-2.0.4.min.js`, license in `static/vendor/htmx-2.0.4.LICENSE`
- **SHA-256:** `e209dda5c8235479f3166defc7750e1dbcd5a5c1808b7792fc2e6733768fb447`

## Fonts

### Schibsted Grotesk (variable, Latin subset)

- **Author:** The Schibsted-Grotesk Project Authors
- **License:** SIL Open Font License 1.1
- **Source:** https://github.com/schibsted/schibsted-grotesk (woff2 via Fontsource)
- **Files:** `static/fonts/schibsted-grotesk.woff2`, license in `static/fonts/schibsted-grotesk.OFL.txt`

## Updating a vendored file

1. Download the new version from npm (unpkg or jsDelivr) and compare its SHA-256 across both CDNs.
2. Save it with the version in the name (e.g. `static/vendor/htmx-2.0.5.min.js`), delete the old file, and update the `<script>` tag in `templates/base.html`.
3. Replace the license file and the version/SHA in this document.

# Interface localization

French is the default interface language. The public `GET /language?lang=en&return_to=/path` endpoint selects English; `lang=fr` restores French. It accepts only local return paths and persists a host-only, HttpOnly, SameSite=Lax cookie for one year. TLS requests receive a Secure cookie. No organization or authentication preference is changed.

The HTTP middleware attaches the language to the request context and emits `Content-Language`. Templ pages declare the matching HTML `lang`; full pages, printable sales documents and HTMX fragments use the same context. The language switch does a normal navigation so that the whole page changes together. Native browser validation messages follow the browser's own language.

`internal/platform/locale/en.json` contains the English catalog. French controlled copy is the key. `pages.T` translates interface text and accessible labels, and `uiText` translates server validation messages. Unknown strings fall back to their source. Whitespace around translated text is retained. Preserve format placeholders when adding a translation. Tests check template catalog coverage, placeholders, safe redirection, persistence, public pricing and unchanged business values.

Never pass organization names, invoice numbers, partner names, product descriptions, units, extracted OCR, model output or financial values to the translator. Downloaded invoice JSON remains structured business data with stable English field names. The displayed monthly pricing is marketing copy; this change does not implement payment collection or enforce invoice allowances.

## Verification on 2026-10-18

Templ generation, Tailwind build, `go test ./...` and `go vet ./...` passed. Go tests cover catalog completeness for literal template keys, unchanged format placeholders, language isolation, encoded unsafe return paths, pricing in both languages, localized CSRF errors and unchanged business data. Rendering tests protect translated labels adjacent to icon components.

The actual PostgreSQL-backed application ran on ERP port 18080 and extractor port 18091. A dedicated headless Chrome session checked 48 page/language/viewport combinations: public French/English pages at 1440, 1024, 390 and 320 pixels, and authenticated English workspace routes at 1440, 390 and 320 pixels. Checks found no page overflow, failed images, hidden language controls or browser exceptions. Login retained English, dashboard HTMX fragments were English, three existing printable sales documents used English, and financial dashboard values were unchanged. Screenshots confirmed the pricing section and authentication layouts on desktop/mobile. Impeccable reported no findings for the edited UI sources. Detailed local artifacts remain under ignored `var/evaluations/` and `var/localization-verification.json`.

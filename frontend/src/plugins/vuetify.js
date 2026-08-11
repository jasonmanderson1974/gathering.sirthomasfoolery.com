import "vuetify/styles"
import { createVuetify } from "vuetify"
import { aliases, mdi } from "vuetify/iconsets/mdi"
import tailwind from "../../tailwind.config"

const c = tailwind.theme.colors
const screens = tailwind.theme.screens
const px = (value) => parseInt(value, 10)

export default createVuetify({
  // The Fellowship — dark vintage gentleman's-club theme.
  //
  // Colours are still sourced from tailwind.config.js rather than duplicated,
  // so the Vuetify palette and the utility classes cannot drift apart.
  theme: {
    defaultTheme: "dark",
    themes: {
      dark: {
        dark: true,
        colors: {
          primary: c.brass,
          secondary: c["green-felt"],
          accent: c["brass-bright"],
          error: c.oxblood,
          info: c.brass,
          success: c.brass,
          // In Vuetify 2 these two were custom keys that only emitted CSS
          // variables; in Vuetify 3 they are real theme keys and actually
          // paint. The page backdrop still comes from `.v-application` in
          // index.css — these just stop components defaulting to a light
          // surface underneath it.
          background: c["wood-deep"],
          surface: c.leather,
        },
      },
      light: {
        dark: false,
        colors: {
          primary: c.brass,
          error: c.oxblood,
        },
      },
    },
  },

  // MDI arrives as a *webfont* — the `@mdi/font` package, imported in main.js
  // and emitted by webpack (it used to be a CDN `<link>`; L8). So this is the
  // font icon set (`mdi`), not `mdi-svg`, which wants SVG paths from `@mdi/js`
  // instead. Get it wrong and all 69 `mdi-*` names in the app render as blank
  // squares with nothing logged.
  icons: {
    defaultSet: "mdi",
    aliases,
    sets: { mdi },
  },

  display: {
    // Vuetify 2's `thresholds` were UPPER bounds (xs meant "narrower than
    // xs"); Vuetify 3's are LOWER bounds (sm means "sm and wider"). The old
    // config read { xs: 640, sm: 768, md: 1024, lg: 1280 }, which described
    // exactly Tailwind's own scale one step over — so rather than translate it
    // by hand, take Tailwind's screens directly. `isPhone` (name === "xs")
    // still means "narrower than 640", as before.
    thresholds: {
      xs: 0,
      sm: px(screens.sm), // 640
      md: px(screens.md), // 768
      lg: px(screens.lg), // 1024
      xl: px(screens.xl), // 1280
      xxl: px(screens["2xl"]), // 1536
    },
  },
})

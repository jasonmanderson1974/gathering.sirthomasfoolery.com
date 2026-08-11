const colors = require("tailwindcss/colors")

module.exports = {
  content: [
    "./index.html",
    "./public/**/*.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  important: true,
  theme: {
    extend: {
      fontSize: {
        xs: ["0.813rem", "1rem"],
      },
      fontFamily: {
        // The Fellowship — all-serif gentleman's-club type.
        // The `Variable` suffix is the family name the self-hosted variable cut
        // actually declares (@fontsource-variable/*, imported in src/main.js) —
        // it is not decoration, and dropping it silently falls through to the
        // generic serif with nothing logged (L9).
        display: ['"Cinzel Variable"', "serif"], // spaced all-caps titles
        head: ['"Cormorant Garamond Variable"', "serif"], // ornamental italic heads
        body: ['"EB Garamond Variable"', "serif"], // running text
      },
    },
    colors: {
      transparent: "transparent",
      current: "currentColor",
      // ---- The Fellowship palette (vintage club · brass & green) ----
      "wood-deep": "#1c1410", // near-black walnut, page base
      wood: "#241a13", // panel wood
      leather: "#2e2117", // raised surfaces
      "green-felt": "#16261d", // billiard-table green accent
      "green-deep": "#0f1c15",
      brass: "#c9a44c", // primary gold/brass
      "brass-bright": "#e3c578", // highlight gold
      "brass-dim": "#8a7333", // hairlines, borders
      parchment: "#ede4d3", // primary text
      "parchment-dim": "#b8ad97", // secondary text
      oxblood: "#6e2b2b", // rare warning/danger
      // ---- legacy Timeful tokens (retained during phased reskin) ----
      "pale-green": "#CDEBDC",
      "light-green": "#29BC68",
      "ligher-green": "#EBF7EF",
      green: "#00994C",
      "dark-green": "#1C7D45",
      "darkest-green": "#007F36",
      "light-blue": "#53A2FF",
      blue: "#006BE8",
      orange: "#E5A800",
      yellow: "#FFE8B8",
      "dark-yellow": "#997700",
      white: "#FFFFFF",
      "off-white": "#F2F2F2",
      black: "#000000",
      gray: "#BDBDBD",
      "dark-gray": "#6B6B6B",
      "very-dark-gray": "#4F4F4F",
      "light-gray": "#f3f4f6",
      "light-gray-stroke": "#dfdfdf",
      "avail-green": colors.emerald, // The green used for marking availability
      red: "#DB1616",
    },
    screens: {
      sm: "640px",
      md: "768px",
      mdlg: "896px",
      lg: "1024px",
      xl: "1280px",
      "2xl": "1536px",
      "publift-s": "755px",
      "publift-m": "995px",
      "publift-l": "1225px",
      "publift-xl": "1475px",
    },
  },
  plugins: [],
  prefix: "tw-",
  safelist: [],
}

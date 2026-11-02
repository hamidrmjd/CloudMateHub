import preset from "@cloudmatehub/design-system/tailwind.preset";
import containerQueries from "@tailwindcss/container-queries";

/** @type {import('tailwindcss').Config} */
export default {
  presets: [preset],
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
    "../../packages/design-system/**/*.{ts,tsx}",
  ],
  theme: { extend: { colors: { primary: { DEFAULT: "#13b8a6", 50: "#13b8a60d", 100: "#13b8a61a", 200: "#13b8a633", 300: "#13b8a64d", 400: "#13b8a680", 500: "#13b8a6", 600: "#0e9488", 700: "#0f766e" } } } },
  plugins: [containerQueries],
};

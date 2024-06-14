/** @type {import('tailwindcss').Config} */
module.exports = {
  // corePlugins: {
  //   preflight: false,
  // },
  content: {
    relative: true,
    files: [
      "./frontend/**/*.{html,js}",
    ],
  },
  theme: {
    extend: {},
  },
  plugins: [],
}


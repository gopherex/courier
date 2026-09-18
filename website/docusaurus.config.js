const { themes } = require('prism-react-renderer');
/** @type {import('@docusaurus/types').Config} */
module.exports = {
  title: 'Courier', tagline: 'Notification delivery for trusted services',
  favicon: 'img/favicon.svg', url: 'https://gopherex.github.io', baseUrl: '/courier/',
  organizationName: 'gopherex', projectName: 'courier', trailingSlash: false,
  onBrokenLinks: 'throw',
  markdown: { hooks: { onBrokenMarkdownLinks: 'throw' } },
  i18n: { defaultLocale: 'en', locales: ['en'] },
  presets: [['classic', {
    docs: { routeBasePath: '/', sidebarPath: require.resolve('./sidebars.js'), editUrl: 'https://github.com/gopherex/courier/edit/master/website/' },
    blog: false, theme: { customCss: require.resolve('./src/css/custom.css') },
  }]],
  themeConfig: {
    colorMode: { defaultMode: 'dark', respectPrefersColorScheme: true },
    navbar: { title: 'Courier', logo: { alt: 'Courier', src: 'img/logo.svg' }, items: [
      { type: 'docSidebar', sidebarId: 'docs', label: 'Docs', position: 'left' },
      { to: '/rest-api/overview', label: 'API', position: 'left' },
      { to: '/sdk/typescript', label: 'SDK', position: 'left' },
      { href: 'https://github.com/gopherex/courier', label: 'GitHub', position: 'right' },
    ] },
    footer: { style: 'dark', copyright: 'Courier · Gopher-EX · MIT License' },
    prism: { theme: themes.github, darkTheme: themes.vsDark, additionalLanguages: ['bash','json','go','yaml'] },
  },
};

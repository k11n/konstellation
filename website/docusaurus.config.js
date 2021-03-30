const images = require('remark-images')
const emoji = require('remark-emoji')
const rehypeTruncate = require('rehype-truncate');

var repoUrl = 'https://github.com/k11n/konstellation'
var signupUrl = 'https://forms.gle/Eh9je8GmS7NRSXf69'

module.exports = {
  title: 'Konstellation',
  tagline: '',
  url: 'https://konstellation.dev',
  baseUrl: '/',
  favicon: 'img/favicon.ico',
  organizationName: 'k11n', // Usually your GitHub org/user name.
  projectName: 'konstellation', // Usually your repo name.
  customFields: {
    signupUrl: signupUrl,
  },
  themeConfig: {
    algolia: {
      apiKey: 'e5335e3bf7e1b015bde7a9d2fb280bb4',
      indexName: 'konstellation',
      searchParameters: {}, // Optional (if provided by Algolia)
    },
    navbar: {
      title: 'Konstellation',
      logo: {
        alt: 'Konstellation Logo',
        src: 'img/logo_dark.png',
        srcDark: 'img/logo_dark.png',
      },
      items: [
        {
          to: 'docs/konstellation/introduction',
          activeBasePath: 'docs',
          label: 'Docs',
          position: 'left',
        },
        {
          href: repoUrl,
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    colorMode: {
      defaultMode: 'dark',
      // disableSwitch: true,
      respectPrefersColorScheme: false,
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Introduction',
          items: [
            {
              label: 'Intro',
              to: 'docs/konstellation/introduction',
            },
            {
              label: 'Why Konstellation',
              to: 'docs/konstellation/why',
            },
            {
              label: 'Design Principles',
              to: 'docs/konstellation/principles',
            },
          ],
        },
        {
          title: 'Docs',
          items: [
            {
              label: 'Getting Started',
              to: 'docs/',
            },
            {
              label: 'Working with Apps',
              to: 'docs/apps/basics',
            },
            {
              label: 'Cluster Operation',
              to: 'docs/clusters/creation',
            },
            {
              label: 'Reference',
              to: 'docs/reference/manifest',
            },
          ],
        },
        {
          title: 'More',
          items: [
            {
              label: 'GitHub',
              href: repoUrl,
            },
            // {
            //   label: 'Support',
            //   to: 'docs/support',
            // }
          ],
        },
      ],
      // Copyright © ${new Date().getFullYear()} Konstellation, LLC.
      copyright: `Apache 2.0 Licensed.`,
    },
    // announcementBar: {
    //   id: 'private_beta',
    //   content: `Konstellation is currently in private beta, interested in early access? <a href="${signupUrl}" target="_blank">Sign up here</a>`,
    //   backgroundColor: '#141e59',
    //   textColor: '#FFFFFF',
    // },
  },
  plugins: [
    'docusaurus-plugin-sass',
  ],
  presets: [
    [
      '@docusaurus/preset-classic',
      {
        docs: {
          sidebarPath: require.resolve('./sidebars.js'),
          // Please change this to your repo.
          editUrl:
            'https://github.com/k11n/konstellation/edit/master/website/',
          remarkPlugins: [images, emoji],
          // rehypePlugins: [rehypeTruncate],
        },
        theme: {
          customCss: require.resolve('./src/css/app.scss'),
        },
        sitemap: {
          cacheTime: 6000 * 1000, // 600 sec - cache purge period
          changefreq: 'weekly',
          priority: 0.5,
        },
      },
    ],
  ],
};

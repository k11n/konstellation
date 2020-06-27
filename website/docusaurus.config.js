const images = require('remark-images')
const emoji = require('remark-emoji')
const rehypeTruncate = require('rehype-truncate');

var repoUrl = 'https://github.com/k11n/konstellation'

module.exports = {
  title: 'Konstellation',
  tagline: 'Open source application platform for Kubernetes',
  url: 'https://konstellation.dev',
  baseUrl: '/',
  favicon: 'img/favicon.ico',
  organizationName: 'k11n', // Usually your GitHub org/user name.
  projectName: 'konstellation', // Usually your repo name.
  themeConfig: {
    navbar: {
      title: 'Konstellation',
      logo: {
        alt: 'Konstellation Logo',
        src: 'img/logo_light.png',
        srcDark: 'img/logo_dark.png',
      },
      links: [
        {
          to: 'docs/',
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
    footer: {
      style: 'dark',
      links: [
        // {
        //   title: 'Docs',
        //   items: [
        //     {
        //       label: 'Style Guide',
        //       to: 'docs/',
        //     },
        //     {
        //       label: 'Second Doc',
        //       to: 'docs/doc2/',
        //     },
        //   ],
        // },
        // {
        //   title: 'Community',
        //   items: [
        //     {
        //       label: 'Stack Overflow',
        //       href: 'https://stackoverflow.com/questions/tagged/docusaurus',
        //     },
        //     {
        //       label: 'Discord',
        //       href: 'https://discordapp.com/invite/docusaurus',
        //     },
        //     {
        //       label: 'Twitter',
        //       href: 'https://twitter.com/docusaurus',
        //     },
        //   ],
        // },
        // {
        //   title: 'More',
        //   items: [
        //     {
        //       label: 'Blog',
        //       to: 'blog',
        //     },
        //     {
        //       label: 'GitHub',
        //       href: repoUrl,
        //     },
        //   ],
        // },
      ],
      copyright: `Apache 2.0 Licensed. Copyright © ${new Date().getFullYear()} David Zhao`,
    },
  },
  presets: [
    [
      '@docusaurus/preset-classic',
      {
        docs: {
          // It is recommended to set document id as docs home page (`docs/` path).
          homePageId: 'konstellation/introduction',
          sidebarPath: require.resolve('./sidebars.js'),
          // Please change this to your repo.
          editUrl:
            'https://github.com/k11n/konstellation/edit/master/website/',
          remarkPlugins: [images, emoji],
          rehypePlugins: [rehypeTruncate],
        },
        theme: {
          customCss: require.resolve('./src/css/custom.css'),
        },
      },
    ],
  ],
};

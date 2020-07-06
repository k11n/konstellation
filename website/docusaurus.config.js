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
            {
              label: 'Join Slack',
              href: '1',
            },
            {
              label: 'Support',
              href: '2',
            }
          ],
        },
      ],
      // Copyright © ${new Date().getFullYear()} Konstellation, LLC.
      copyright: `Apache 2.0 Licensed.`,
    },
  },
  presets: [
    [
      '@docusaurus/preset-classic',
      {
        docs: {
          // It is recommended to set document id as docs home page (`docs/` path).
          homePageId: 'getting-started/installation',
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

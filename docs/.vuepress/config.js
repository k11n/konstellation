module.exports = {
  title: "Konstellation",
  themeConfig: {
    nav: [
      { text: 'Guide', link: '/guide/' },
      // { text: 'Reference', link: '/reference/' },
      { text: 'Github', link: 'https://github.com/k11n/konstellation' },
    ],
    sidebar: {
      '/guide/': [
        'why',
        '',
        'apps',
        'clusters',
        'networking',
      ],
      '/reference/': [
        '',
      ]
    }
  }
}

module.exports = {
  title: "Konstellation",
  themeConfig: {
    logo: '/konstellation-icon.png',
    nav: [
      { text: 'Guide', link: '/guide/' },
      { text: 'Github', link: 'https://github.com/k11n/konstellation' },
    ],
    sidebar: {
      '/guide/': [
        'principles',
        '',
        'apps',
        'clusters',
        'manifest',
      ],
      '/reference/': [
        '',
      ]
    }
  }
}

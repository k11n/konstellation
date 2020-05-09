module.exports = {
  title: "Konstellation Docs",
  themeConfig: {
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Guide', link: '/guide/' },
      { text: 'Reference', link: '/reference/' },
    ],
    // sidebar: [
    //   {
    //     title: 'Guide',
    //     path: '/guide/',
    //     collapsable: false,
    //     sidebarDepth: 2,
    //   },
    //   {
    //     title: 'Reference',
    //     path: '/reference/',
    //     collapsable: false,
    //     sidebarDepth: 2,
    //   }
    // ]
    sidebar: {
      '/guide/': [
        '',
        'one'
      ],
      '/reference/': [
        '',
      ]
    }
  }
}

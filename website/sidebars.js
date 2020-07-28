module.exports = {
  default: [
    {
      type: 'category',
      label: 'Konstellation',
      collapsed: false,
      items: ['konstellation/introduction', 'konstellation/why', 'konstellation/principles'],
    },
    {
      type: 'category',
      label: 'Getting Started',
      collapsed: false,
      items: ['getting-started/installation', 'getting-started/deploy'],
    },
    {
      type: 'category',
      label: 'Working With Apps',
      collapsed: false,
      items: [
        'apps/basics',
        'apps/configuration',
        'apps/develop',
        'apps/services',
        'apps/monitoring',
      ],
    },
    {
      type: 'category',
      label: 'Cluster Operation',
      collapsed: false,
      items: [
        'clusters/creation',
        'clusters/users',
        'clusters/networking',
        'clusters/upgrading',
        'clusters/migration',
      ],
    },
    {
      type: 'category',
      label: 'Reference',
      items: [
        'reference/manifest',
        'reference/linkedserviceaccount',
      ],
    },
  ],
};

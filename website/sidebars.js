module.exports = {
  default: [
    {
      type: 'category',
      label: 'Konstellation',
      items: ['konstellation/introduction', 'konstellation/why', 'konstellation/principles'],
    },
    {
      type: 'category',
      label: 'Getting Started',
      items: ['getting-started/installation', 'getting-started/deploy'],
    },
    {
      type: 'category',
      label: 'Working With Apps',
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
      items: [
        'clusters/creation',
        'clusters/users',
        'clusters/upgrading',
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
    {
      type: 'doc',
      id: 'support',
    },
  ],
};

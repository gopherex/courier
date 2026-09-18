module.exports = { docs: [
  'intro', 'quickstart',
  { type: 'category', label: 'Concepts', items: ['concepts/overview','concepts/delivery','concepts/preferences'] },
  { type: 'category', label: 'Guides', items: ['guides/admin','guides/templates','guides/providers','guides/dead-letters'] },
  { type: 'category', label: 'REST API', items: ['rest-api/overview','rest-api/errors'] },
  { type: 'category', label: 'SDK', items: ['sdk/go','sdk/typescript'] },
  { type: 'category', label: 'Self-hosting', items: ['self-hosting/docker','self-hosting/configuration','self-hosting/operations'] },
  { type: 'category', label: 'Development', items: ['development/testing','development/releasing'] },
] };

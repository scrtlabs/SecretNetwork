module.exports = {
    description: 'Privacy is a Public Good',
    base: '/SecretNetwork/',
    head: [
        ['link', { rel: 'icon', href: '/logo.png' }]
      ],
    themeConfig: {
        logo: '/logo.png',
        nav: [
            { text: 'Docs', link: '/overview'},
            { text: 'Website', link: 'https://scrt.network' },
            { text: 'Blog', link: 'https://blog.scrt.network' },
            { text: 'Chat', link: 'https://chat.scrt.network' },
            { text: 'Forum', link: 'https://forum.scrt.network' },
            { text: 'Twitter', link: 'https://twitter.com/SecretNetwork' }
          ],
        sidebar: [
            {
                title: 'Overview',   // required
                path: '/overview',      // optional, link of the title, which should be an absolute path and must exist
                collapsable: true, // optional, defaults to true
                sidebarDepth: 0,    // optional, defaults to 1
                children: [
                  '/protocol/architecture',
                  '/protocol/roadmap',
                  '/ledger-nano-s'
                ]
            },
            {
                title: 'App Developers',   // required
                path: '/dev/developers',      // optional, link of the title, which should be an absolute path and must exist
                collapsable: true, // optional, defaults to true
                sidebarDepth: 0,    // optional, defaults to 1
                children: [
                  '/dev/for-blockchain-devs',
                  '/dev/contract-dev-guide',
                  '/secretcli'
                ]
            },
            {
                title: 'Node Operators', // required
                path: '/validators-and-full-nodes/secret-nodes', // optional, link of the title, which should be an absolute path and must exist
                collapsable: true, // optional, defaults to true
                sidebarDepth: 0, // optional, defaults to 1
                children: [
                  '/validators-and-full-nodes/setup-sgx',
                  '/validators-and-full-nodes/run-full-node-mainnet',
                  '/validators-and-full-nodes/join-validator-mainnet',
                  '/validators-and-full-nodes/backup-a-validator',
                  '/validators-and-full-nodes/migrate-a-validator',
                  '/validators-and-full-nodes/sentry-nodes'
                ]
            },
            {
                title: 'Protocol',   // required
                path: '/protocol/intro',      // optional, link of the title, which should be an absolute path and must exist
                collapsable: true, // optional, defaults to true
                sidebarDepth: 0,    // optional, defaults to 1
                children: [
                  '/protocol/components',
                  '/protocol/encryption-specs',
                  '/protocol/transactions',
                  '/protocol/governance',
                  '/protocol/sgx'
                ]
            }
          ]
      }
  }